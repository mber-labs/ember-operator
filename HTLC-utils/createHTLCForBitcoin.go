package HTLC

import (
	"encoding/hex"
	"fmt"
	"log"

	"os/exec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcutil"
)

func CreateHTLCForBitcoin(sha256Hex, makerBTCPubKeyHex string) (string, string) {
	resolverWIFStr := "cNN2Rn61aVDRuYeNNu4v4zUF13zfmWunsmoSDdYhDrkL7CFVDvZa"

	fmt.Println("🔗 Creating HTLCs...")
	fmt.Println("sha256Hex:", sha256Hex)
	fmt.Println("BTC PubKey:", makerBTCPubKeyHex)
	fmt.Println("resolver WIF:", resolverWIFStr)

	// Your HTLC logic will go here

    lockBlock, err := getCurrentBlockHeight()
	if err != nil {
		log.Fatal("❌ Failed to get block height:", err)
	}
	lockBlock += 10
	fmt.Println("🔒 Lock until block height:", lockBlock)

	// Decode Alice (resolver) WIF
	resolverWIF, err := btcutil.DecodeWIF(resolverWIFStr)
	if err != nil {
		log.Fatal("❌ Failed to decode Alice's WIF:", err)
	}
	alicePubKeyBytes := resolverWIF.PrivKey.PubKey().SerializeCompressed()

	// Decode maker BTC pubkey (hex)
	makerBTCPubKeyBytes, err := hex.DecodeString(makerBTCPubKeyHex)
	if err != nil {
		log.Fatal("❌ Invalid maker BTC pubkey hex:", err)
	}

	// Decode hashLock (hex) and hash160 it
	sha256, err := hex.DecodeString(sha256Hex)
	if err != nil {
		log.Fatal("❌ Invalid hashLock hex:", err)
	}

	// Build redeem script
	builder := txscript.NewScriptBuilder().
		AddOp(txscript.OP_IF).
		AddOp(txscript.OP_SHA256).
		AddData(sha256).
		AddOp(txscript.OP_EQUALVERIFY).
		AddData(makerBTCPubKeyBytes).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ELSE).
		AddInt64(int64(lockBlock)).
		AddOp(txscript.OP_CHECKSEQUENCEVERIFY).
		AddOp(txscript.OP_DROP).
		AddData(alicePubKeyBytes).
		AddOp(txscript.OP_CHECKSIG).
		AddOp(txscript.OP_ENDIF)

	redeemScript, err := builder.Script()
	if err != nil {
		log.Fatal("❌ Failed to build redeem script:", err)
	}

	// Generate P2SH address
	address, err := btcutil.NewAddressScriptHash(redeemScript, &chaincfg.RegressionNetParams)
	if err != nil {
		log.Fatal("❌ Failed to create P2SH address:", err)
	}

	fmt.Println("🔐 Redeem Script (hex):", hex.EncodeToString(redeemScript))
	fmt.Println("📬 P2SH Address        :", address.EncodeAddress())

	return hex.EncodeToString(redeemScript), address.EncodeAddress()
}

func getCurrentBlockHeight() (int64, error) {
	cmd := exec.Command("bitcoin-cli", "-regtest", "getblockcount")
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	var height int64
	_, err = fmt.Sscanf(string(output), "%d", &height)
	if err != nil {
		return 0, err
	}
	return height, nil
}