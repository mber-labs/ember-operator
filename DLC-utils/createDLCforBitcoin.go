package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"log"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
)

// ---------- Types & Params ----------

type Keys struct {
	BorrowerPub []byte // 33-byte compressed secp256k1 public key
	LenderPub   []byte // 33-byte compressed secp256k1 public key
}

type FundingUTXO struct {
	TxIDHex string
	Vout    uint32
	Amount  int64 // sats
}

type Destinations struct {
	BorrowerP2WPKH []byte // borrower witness program (20-byte hash) for P2WPKH
	LenderP2WPKH   []byte // lender witness program (20-byte hash) for P2WPKH
}

type Deal struct {
	TotalCollateral   int64 // sats locked in funding
	LoanAmount        int64 // sats owed to lender on expiry path
	ExpiryUnix        int64 // absolute unix time for refund/expiry paths (nLockTime)
	HashH2            []byte // 32-byte sha256(s2)
	HashlockCollateral bool   // true: also prepare a dedicated HTLC spend for borrower via preimage
	Network           *chaincfg.Params
}

// ---------- Helpers ----------

func must(err error) {
	if err != nil {
		panic(err)
	}
}

// 2-of-2 Multisig witness script (sorted keys recommended in production)
func build2of2WitnessScript(borrowerPub33, lenderPub33 []byte) ([]byte, error) {
	builder := txscript.NewScriptBuilder()
	builder.AddInt64(2).
		AddData(borrowerPub33).
		AddData(lenderPub33).
		AddInt64(2).
		AddOp(txscript.OP_CHECKMULTISIG)
	return builder.Script()
}

// P2WSH address from witnessScript
func p2wshAddr(witnessScript []byte, net *chaincfg.Params) (btcutil.Address, error) {
	wsh := btcutil.Hash160(witnessScript) // This is HASH160; for P2WSH we need SHA256!
	// Correct P2WSH uses SHA256, not HASH160:
	_ = wsh
	sh := txscript.Sha256(witnessScript)
	return btcutil.NewAddressWitnessScriptHash(sh[:], net)
}

// P2WPKH pkScript for a 20-byte witness program
func p2wpkhScript20(program20 []byte) ([]byte, error) {
	if len(program20) != 20 {
		return nil, fmt.Errorf("wpkh program must be 20 bytes")
	}
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0).AddData(program20)
	return builder.Script()
}

// P2WSH pkScript for a given witnessScript (sha256)
func p2wshScript(witnessScript []byte) ([]byte, error) {
	sh := txscript.Sha256(witnessScript)
	builder := txscript.NewScriptBuilder()
	builder.AddOp(txscript.OP_0).AddData(sh[:])
	return builder.Script()
}

// Make a standard input from a funding outpoint
func makeFundingInput(f FundingUTXO) (*wire.OutPoint, *wire.TxIn, error) {
	txHash, err := chainhashFromHex(f.TxIDHex)
	if err != nil {
		return nil, nil, err
	}
	op := wire.NewOutPoint(txHash, f.Vout)
	in := wire.NewTxIn(op, nil, nil)
	return op, in, nil
}

func chainhashFromHex(h string) (*wire.Hash, error) {
	b, err := hex.DecodeString(h)
	if err != nil || len(b) != 32 {
		return nil, fmt.Errorf("bad txid")
	}
	var rev [32]byte
	// Bitcoin displays txid big-endian, wire.Hash expects little-endian
	for i := 0; i < 32; i++ {
		rev[i] = b[31-i]
	}
	var out wire.Hash
	copy(out[:], rev[:])
	return &out, nil
}

// ---------- CET Builders ----------

// CET: all collateral to borrower (hash(s2)=h2 off-chain trigger OR optional HTLC on-chain)
func buildCETCollateralToBorrower(
	f FundingUTXO,
	fundingScript []byte,
	totalCollateral int64,
	dest Destinations,
) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(wire.TxVersion)
	_, in, err := makeFundingInput(f)
	if err != nil {
		return nil, err
	}
	tx.AddTxIn(in)

	// Single output to borrower
	pkScript, err := p2wpkhScript20(dest.BorrowerP2WPKH)
	if err != nil {
		return nil, err
	}

	// Simple fee model for template: subtract a flat fee (e.g. 300 sats)
	fee := int64(300)
	if totalCollateral <= fee {
		return nil, fmt.Errorf("insufficient amount for fee")
	}
	out := wire.NewTxOut(totalCollateral-fee, pkScript)
	tx.AddTxOut(out)

	// No locktime for immediate execution
	tx.LockTime = 0

	// Set input sequence to final (0xffffffff)
	tx.TxIn[0].Sequence = 0xffffffff

	// NOTE: You must sign this tx’s input with both parties’ keys against the P2WSH fundingScript.
	_ = fundingScript
	return tx, nil
}

// CET: all collateral to lender (oracle attests LTV risky)
func buildCETCollateralToLender(
	f FundingUTXO,
	fundingScript []byte,
	totalCollateral int64,
	dest Destinations,
) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(wire.TxVersion)
	_, in, err := makeFundingInput(f)
	if err != nil {
		return nil, err
	}
	tx.AddTxIn(in)

	pkScript, err := p2wpkhScript20(dest.LenderP2WPKH)
	if err != nil {
		return nil, err
	}

	fee := int64(300)
	if totalCollateral <= fee {
		return nil, fmt.Errorf("insufficient amount for fee")
	}
	out := wire.NewTxOut(totalCollateral-fee, pkScript)
	tx.AddTxOut(out)

	tx.LockTime = 0
	tx.TxIn[0].Sequence = 0xffffffff

	_ = fundingScript
	return tx, nil
}

// CET: on loan expiry — pay loanAmount to lender, remainder to borrower
func buildCETExpirySplit(
	f FundingUTXO,
	fundingScript []byte,
	totalCollateral int64,
	loanAmount int64,
	dest Destinations,
) (*wire.MsgTx, error) {

	if loanAmount <= 0 || loanAmount >= totalCollateral {
		return nil, fmt.Errorf("loanAmount invalid")
	}

	tx := wire.NewMsgTx(wire.TxVersion)
	_, in, err := makeFundingInput(f)
	if err != nil {
		return nil, err
	}
	tx.AddTxIn(in)

	fee := int64(400) // two outputs
	if totalCollateral <= fee {
		return nil, fmt.Errorf("insufficient amount for fee")
	}

	// Output #1 to lender: loanAmount - half fees (rough split)
	lenderOut := loanAmount - fee/2
	if lenderOut <= 0 {
		return nil, fmt.Errorf("loanAmount too small after fee")
	}

	// Output #2 to borrower: remainder
	borrowerOut := totalCollateral - lenderOut - fee/2
	if borrowerOut <= 0 {
		return nil, fmt.Errorf("remainder non-positive")
	}

	lenderScript, err := p2wpkhScript20(dest.LenderP2WPKH)
	if err != nil {
		return nil, err
	}
	borrowerScript, err := p2wpkhScript20(dest.BorrowerP2WPKH)
	if err != nil {
		return nil, err
	}

	tx.AddTxOut(wire.NewTxOut(lenderOut, lenderScript))
	tx.AddTxOut(wire.NewTxOut(borrowerOut, borrowerScript))

	// If you want to enforce “expiry” on-chain, you typically use a REFUND tx with nLockTime,
	// not a CET locktime. CETs usually execute immediately once outcome is known.
	// Keep locktime 0; enforce via business logic/UX off-chain.
	tx.LockTime = 0
	tx.TxIn[0].Sequence = 0xffffffff

	_ = fundingScript
	return tx, nil
}

// Refund TX: after absolute time `expiryUnix`, send all to lender (safety fallback)
func buildRefundTx(
	f FundingUTXO,
	fundingScript []byte,
	totalCollateral int64,
	dest Destinations,
	expiryUnix int64,
) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(wire.TxVersion)
	_, in, err := makeFundingInput(f)
	if err != nil {
		return nil, err
	}
	tx.AddTxIn(in)

	outScript, err := p2wpkhScript20(dest.LenderP2WPKH)
	if err != nil {
		return nil, err
	}

	fee := int64(300)
	if totalCollateral <= fee {
		return nil, fmt.Errorf("insufficient for fee")
	}
	tx.AddTxOut(wire.NewTxOut(totalCollateral-fee, outScript))

	// Absolute CLTV: nLockTime set to block height or timestamp.
	// Here we use timestamp-style locktime: value >= 500,000,000.
	if expiryUnix < 500_000_000 {
		return nil, fmt.Errorf("expiry must be unix timestamp >= 500,000,000")
	}
	tx.LockTime = uint32(expiryUnix)

	// Sequence must be < 0xffffffff to make CLTV enforceable
	tx.TxIn[0].Sequence = 0xfffffffe

	_ = fundingScript
	return tx, nil
}

// Optional: separate HTLC P2WSH output that lets borrower claim with preimage s2.
// This demonstrates an *on-chain* hash check (independent of DLC adaptor mechanism).
func buildHashlockWitnessScript(borrowerPub33 []byte, h2 []byte) ([]byte, error) {
	if len(h2) != 32 {
		return nil, fmt.Errorf("h2 must be 32 bytes sha256")
	}
	builder := txscript.NewScriptBuilder()
	// OP_SHA256 <h2> OP_EQUALVERIFY <borrowerPub> OP_CHECKSIG
	builder.AddOp(txscript.OP_SHA256).
		AddData(h2).
		AddOp(txscript.OP_EQUALVERIFY).
		AddData(borrowerPub33).
		AddOp(txscript.OP_CHECKSIG)
	return builder.Script()
}

// Spend the HTLC output to borrower (requires s2 preimage + borrower sig)
func buildSpendFromHashlock(
	htlcOut FundingUTXO,
	htlcWitnessScript []byte,
	amount int64,
	dest Destinations,
) (*wire.MsgTx, error) {

	tx := wire.NewMsgTx(wire.TxVersion)
	_, in, err := makeFundingInput(htlcOut)
	if err != nil {
		return nil, err
	}
	tx.AddTxIn(in)
	pkScript, err := p2wpkhScript20(dest.BorrowerP2WPKH)
	if err != nil {
		return nil, err
	}
	fee := int64(300)
	if amount <= fee {
		return nil, fmt.Errorf("insufficient for fee")
	}
	tx.AddTxOut(wire.NewTxOut(amount-fee, pkScript))

	tx.LockTime = 0
	tx.TxIn[0].Sequence = 0xffffffff

	// Signing/witness: <sig> <s2> <witnessScript>
	_ = htlcWitnessScript
	return tx, nil
}

func mustTxHex(tx *wire.MsgTx) string {
	var buf bytes.Buffer
	_ = tx.Serialize(&buf)
	return hex.EncodeToString(buf.Bytes())
}
