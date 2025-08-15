package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	ethAddress      string
	privateKey      *ecdsa.PrivateKey
	ethRPCUrl       = "http://localhost:8545"
	ethWSRPCUrl     = "ws://localhost:8545"
	contractAddress = common.HexToAddress("0x5FbDB2315678afecb367f032d93F642f64180aa3")
	emberManagerABI = `[{"inputs":[],"name":"registerOperator","outputs":[],"stateMutability":"payable","type":"function"}]`
	chainID         = big.NewInt(1337) // Replace with your chain ID
	aggregator      string
)

func helloHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "✅ ETH Address: %s\n", ethAddress)
}

func ethAddressHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte(ethAddress))
}

func registerHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("📤 Registering operator...")
	go func() {
		err := registerOperator()
		if err != nil {
			log.Printf("❌ registerOperator failed: %v\n", err)
		}
	}()
	w.Write([]byte("📤 Registering operator..."))
}

func registerOperator() error {
	client, err := ethclient.Dial(ethRPCUrl)
	if err != nil {
		return fmt.Errorf("failed to connect to Ethereum node: %v", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(emberManagerABI))
	if err != nil {
		return fmt.Errorf("failed to parse ABI: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		return fmt.Errorf("failed to create transactor: %v", err)
	}

	auth.Value = big.NewInt(1e18)   // 1 ETH
	auth.GasLimit = uint64(300000)  // gas limit
	auth.GasPrice = big.NewInt(1e9) // gas price
	nonce, err := client.PendingNonceAt(context.Background(), auth.From)
	if err != nil {
		return fmt.Errorf("failed to get nonce: %v", err)
	}
	auth.Nonce = big.NewInt(int64(nonce))

	data, err := parsedABI.Pack("registerOperator")
	if err != nil {
		return fmt.Errorf("failed to pack data: %v", err)
	}

	tx := types.NewTransaction(nonce, contractAddress, auth.Value, auth.GasLimit, auth.GasPrice, data)
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return fmt.Errorf("failed to sign tx: %v", err)
	}

	err = client.SendTransaction(context.Background(), signedTx)
	if err != nil {
		return fmt.Errorf("failed to send tx: %v", err)
	}

	log.Printf("📤 Sent registerOperator tx: %s\n", signedTx.Hash().Hex())
	return nil
}

func main() {
	var err error
	// privateKey, err = crypto.GenerateKey()

	hexKey := "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	privateKey, err = crypto.HexToECDSA(hexKey)

	if err != nil {
		log.Fatal(err)
	}
	publicKey := privateKey.Public().(*ecdsa.PublicKey)
	ethAddress = crypto.PubkeyToAddress(*publicKey).Hex()

	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	fmt.Printf("✅ ETH Address: %s\n", ethAddress)
	fmt.Printf("🌐 Running at http://localhost:%d\n", port)

	http.HandleFunc("/", helloHandler)
	http.HandleFunc("/register", registerHandler)
	http.HandleFunc("/eth-address", ethAddressHandler)
	http.HandleFunc("/btc-key", storePrivateKey)

	go ListenOperatorSelected(ethWSRPCUrl, ethRPCUrl, contractAddress)

	log.Fatal(http.Serve(listener, nil))
}
