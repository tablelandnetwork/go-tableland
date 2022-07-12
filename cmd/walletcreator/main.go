package main

import (
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	fileName := flag.String("filename", "privatekey.hex", "Filename to store hex representation of private key")
	flag.Parse()

	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}
	privateKeyBytes := crypto.FromECDSA(privateKey)

	if err := os.WriteFile(*fileName, []byte(hexutil.Encode(privateKeyBytes)[2:]), 0o644); err != nil {
		log.Fatalf("writing to file %s: %s", *fileName, err)
	}
	pubk, _ := privateKey.Public().(*ecdsa.PublicKey)
	publicKey := common.HexToAddress(crypto.PubkeyToAddress(*pubk).Hex())

	fmt.Printf("Wallet address %s created\n", publicKey)
	fmt.Printf("Private key saved in %s\n", *fileName)
}
