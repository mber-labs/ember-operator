package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type RequestData struct {
	PrivateKey string `json:"privateKey"`
	Sender     string `json:"sender"`
}

func storePrivateKey(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST allowed", http.StatusMethodNotAllowed)
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil || len(body) == 0 {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var req RequestData
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	// Normalize to lowercase for comparison
	if strings.ToLower(req.Sender) != strings.ToLower(aggregator) {
		http.Error(w, "Unauthorized sender", http.StatusUnauthorized)
		return
	}

	err = os.WriteFile("creds.txt", []byte(req.PrivateKey), 0644)
	if err != nil {
		http.Error(w, "Failed to store key", http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(w, "✅ Private key stored (overwritten)")
}

func sendPrivateKey(ip, ethAddress, pvtKey string) {
	url := fmt.Sprintf("http://%s/btc-key", ip)

	payload := RequestData{
		PrivateKey: pvtKey,
		Sender:     ethAddress,
	}

	body, _ := json.Marshal(payload)

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("❌ Failed to call %s: %v", url, err)
		return
	}
	defer resp.Body.Close()

	fmt.Printf("✅ Sent to %s | Status: %s\n", url, resp.Status)
}
