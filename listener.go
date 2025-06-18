package main

import (
	"context"
	"fmt"
	"log"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/btcsuite/btcd/btcec"
)

const operatorSelectedEventABI = `[{"anonymous":false,"inputs":[{"indexed":false,"internalType":"address","name":"selectedOperator","type":"address"}],"name":"OperatorSelected","type":"event"}]`
const fetchOperatorIPsABI = `[
	{"inputs":[],"name":"getAllOperators","outputs":[{"internalType":"address[]","name":"","type":"address[]"}],"stateMutability":"view","type":"function"},
	{"inputs":[{"internalType":"address","name":"operator","type":"address"}],"name":"getOperatorIP","outputs":[{"internalType":"string","name":"","type":"string"}],"stateMutability":"view","type":"function"}
]`
const fetchLastSelectedOperatorABI = `[
	{"inputs":[],"name":"getLastSelectedOperator","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}
]`

func ListenOperatorSelected(rpcURL string, contractAddress common.Address) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		log.Fatal("Failed to connect to Ethereum client:", err)
	}

	parsedABI, err := abi.JSON(strings.NewReader(operatorSelectedEventABI))
	if err != nil {
		log.Fatal("Failed to parse ABI:", err)
	}

	query := ethereum.FilterQuery{
		Addresses: []common.Address{contractAddress},
	}

	logs := make(chan types.Log)
	sub, err := client.SubscribeFilterLogs(context.Background(), query, logs)
	if err != nil {
		log.Fatal("Failed to subscribe to logs:", err)
	}

	fmt.Println("🟢 Listening for OperatorSelected events...")

	for {
		select {
		case err := <-sub.Err():
			log.Fatal("Subscription error:", err)
		case vLog := <-logs:
			var event struct {
				SelectedOperator common.Address
			}

			err := parsedABI.UnpackIntoInterface(&event, "OperatorSelected", vLog.Data)
			if err != nil {
				log.Println("Failed to unpack event:", err)
				continue
			}

			fmt.Println("📢 Operator Selected:", event.SelectedOperator.Hex())

			if event.SelectedOperator.Hex() == ethAddress {
				// this operator is selected as aggregator
				// initiate DKG process
				privateKey, _ := btcec.NewPrivateKey(btcec.S256())
				fmt.Println("Private Key:", privateKey.D)
				shares := SplitSecret(privateKey.D, 5, 3)
				fmt.Println("Shares:", shares)

				ips, err := FetchOperatorIPs(rpcURL, common.HexToAddress("0xYourContractAddress"))
				if err != nil {
					log.Fatal(err)
				}
				i := 0
				for addr, ip := range ips {
					share := shares[i]
					sendPrivateKey(ip, ethAddress, pvtKeyToString(share))
					fmt.Printf("%s => %s\n", addr.Hex(), ip)
				}
			} else {
				aggregator = event.SelectedOperator.Hex()
			}
		}
	}
}

func FetchOperatorIPs(rpcURL string, contractAddress common.Address) (map[common.Address]string, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return nil, err
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(fetchOperatorIPsABI))
	if err != nil {
		return nil, err
	}

	// Call getAllOperators
	call := func(name string, args ...interface{}) ([]byte, error) {
		data, err := parsedABI.Pack(name, args...)
		if err != nil {
			return nil, err
		}
		return client.CallContract(context.Background(), ethereum.CallMsg{To: &contractAddress, Data: data}, nil)
	}

	out, err := call("getAllOperators")
	if err != nil {
		return nil, err
	}

	var operators []common.Address
	if err := parsedABI.UnpackIntoInterface(&operators, "getAllOperators", out); err != nil {
		return nil, err
	}

	result := make(map[common.Address]string)
	for _, op := range operators {
		out, err := call("getOperatorIP", op)
		if err != nil {
			continue
		}
		var ip string
		if err := parsedABI.UnpackIntoInterface(&ip, "getOperatorIP", out); err == nil {
			result[op] = ip
		}
	}
	return result, nil
}

func FetchLastSelectedOperator(rpcURL string, contractAddress common.Address) (common.Address, error) {
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		return common.Address{}, err
	}
	defer client.Close()

	parsedABI, err := abi.JSON(strings.NewReader(fetchLastSelectedOperatorABI))
	if err != nil {
		return common.Address{}, err
	}

	data, err := parsedABI.Pack("getLastSelectedOperator")
	if err != nil {
		return common.Address{}, err
	}

	msg := ethereum.CallMsg{
		To:   &contractAddress,
		Data: data,
	}

	out, err := client.CallContract(context.Background(), msg, nil)
	if err != nil {
		return common.Address{}, err
	}

	var selected common.Address
	if err := parsedABI.UnpackIntoInterface(&selected, "getLastSelectedOperator", out); err != nil {
		return common.Address{}, err
	}

	return selected, nil
}
