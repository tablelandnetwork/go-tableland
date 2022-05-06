package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spruceid/siwe-go"
)

func main() {
	validFor := flag.Duration("duration", time.Hour*24*365*100, "validity duration (def: 1 year)")
	chainID := flag.Int("chain-id", 69, "chain id (def: 69, Optimism Kovan))")
	flag.Parse()

	if len(os.Args) < 2 {
		log.Fatalf("we expect one argument, use -help")
	}

	privKeyHex := os.Args[1]
	pk, err := crypto.HexToECDSA(privKeyHex)
	if err != nil {
		log.Fatalf("decoding private key: %s", err)
	}

	opts := map[string]interface{}{
		"chainID":        chainID,
		"expirationTime": time.Now().Add(*validFor),
		"nonce":          siwe.GenerateNonce(),
	}
	pubKey := crypto.PubkeyToAddress(pk.PublicKey).Hex()
	msg, err := siwe.InitMessage("Tableland", pubKey, "https://staging.tableland.io", "1", opts)
	if err != nil {
		log.Fatalf("creating siwe message: %s", err)
	}

	payload := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(msg.String()), msg.String())
	hash := crypto.Keccak256Hash([]byte(payload))
	signature, err := crypto.Sign(hash.Bytes(), pk)
	if err != nil {
		log.Fatalf("signing siwe message: %s", err)
	}
	signature[64] += 27

	bearerValue := struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}{Message: msg.String(), Signature: hexutil.Encode(signature)}
	bearer, err := json.Marshal(bearerValue)
	if err != nil {
		log.Fatalf("json marshaling signed siwe: %s", err)
	}

	fmt.Printf("Bearer %s\n\n", base64.StdEncoding.EncodeToString(bearer))
	fmt.Printf("Signed by %s", pubKey)
}
