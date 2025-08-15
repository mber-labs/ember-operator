package HTLC

import (
	"context"
	"crypto/ecdsa"
	"encoding/hex"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func CreateHTLCForEthereum(
	clientURL string,
	privateKeyHex string,
	contractAddressHex string,
	receiverHex string,
	secretSHA256Hex string,
	timelockSeconds int64,
	amountInWei *big.Int,
) (string, error) {
	// Connect to Ethereum node
	client, err := ethclient.Dial(clientURL)
	if err != nil {
		return "", err
	}
	defer client.Close()

	// Parse private key
	privateKey, err := crypto.HexToECDSA(privateKeyHex)
	if err != nil {
		return "", err
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	fromAddress := crypto.PubkeyToAddress(*publicKey)

	// Nonce, gas, chainID
	nonce, err := client.PendingNonceAt(context.Background(), fromAddress)
	if err != nil {
		return "", err
	}
	gasPrice, err := client.SuggestGasPrice(context.Background())
	if err != nil {
		return "", err
	}
	chainID, err := client.NetworkID(context.Background())
	if err != nil {
		return "", err
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return "", err
	}
	auth.Nonce = big.NewInt(int64(nonce))
	auth.Value = amountInWei
	auth.GasLimit = uint64(300000) // adjust as needed
	auth.GasPrice = gasPrice

	// Load ABI
	htlcAbi, err := abi.JSON(strings.NewReader(string(HTLCRegistryABI)))
	if err != nil {
		return "", err
	}

	// Prepare args
	contractAddress := common.HexToAddress(contractAddressHex)
	receiver := common.HexToAddress(receiverHex)
	secretSHA256, _ := hex.DecodeString(secretSHA256Hex)
	var hashLock [32]byte
	copy(hashLock[:], secretSHA256)

	timelock := big.NewInt(time.Now().Unix() + timelockSeconds)

	// Pack input data
	input, err := htlcAbi.Pack("createHTLC", receiver, hashLock, timelock)
	if err != nil {
		return "", err
	}

	// Create and sign tx
	tx := types.NewTransaction(nonce, contractAddress, auth.Value, auth.GasLimit, auth.GasPrice, input)
	signedTx, err := types.SignTx(tx, types.LatestSignerForChainID(chainID), privateKey)
	if err != nil {
		return "", err
	}

	// Send tx
	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return "", err
	}

	return signedTx.Hash().Hex(), nil
}
