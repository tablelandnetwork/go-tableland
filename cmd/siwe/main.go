package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/siwe"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func main() {
	validFor := flag.Duration("duration", time.Hour*24*365*100, "validity duration (def: 1 year)")
	chainID := flag.Int("chain-id", 69, "chain id (def: 69, Optimism Kovan))")
	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatalf("we expect one argument, use -help")
	}

	privKeyHex := flag.Args()[0]

	w, err := wallet.NewWallet(privKeyHex)
	if err != nil {
		log.Fatalf("decoding private key: %s", err)
	}

	siwe, err := siwe.EncodedSIWEMsg(tableland.ChainID(*chainID), w, *validFor)
	if err != nil {
		log.Fatalf("creating bearer value: %v", err)
	}

	fmt.Printf("%s\n\n", siwe)
	fmt.Printf("Signed by %s\n", w.Address().Hex())
}
