package main

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

func main() {
	ethEndpoint := flag.String("ethendpoint", "", "URL of an Ethereum node API (i.e: Alchemy/Infura)")
	privateKey := flag.String("privatekey", "", "Hex encoded private key of the wallet address")
	flag.Parse()

	if len(flag.Args()) < 1 {
		log.Fatalf("we expect one argument\n./gaspricebumper [flags] <stuck-txn-hash>")
	}
	stuckTxnHash := common.HexToHash(flag.Args()[0])

	pk, err := crypto.HexToECDSA(*privateKey)
	if err != nil {
		log.Fatalf("decoding private key: %s", err)
	}

	conn, err := ethclient.Dial(*ethEndpoint)
	if err != nil {
		log.Fatalf("failed to connect to ethereum endpoint: %s", err)
	}

	newTxnHash, err := bumpTxnFee(conn, pk, stuckTxnHash)
	if err != nil {
		log.Fatalf("bumpint txn fee: %s", err)
	}
	fmt.Printf("The new transaction hash is: %s\n", newTxnHash)
}

func bumpTxnFee(
	conn *ethclient.Client,
	pk *ecdsa.PrivateKey,
	stuckTxnHash common.Hash) (common.Hash, error) {

	ctx := context.Background()

	pendingTxn, isPending, err := conn.TransactionByHash(ctx, stuckTxnHash)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get pending txn from the mempool: %s", err)
	}
	if !isPending {
		return common.Hash{}, fmt.Errorf("the transaction hash %s isn't pending", stuckTxnHash)
	}

	ltxn := &types.LegacyTx{
		Nonce:    pendingTxn.Nonce(),
		GasPrice: big.NewInt(0).Div(big.NewInt(0).Mul(pendingTxn.GasPrice(), big.NewInt(125)), big.NewInt(100)),
		Gas:      pendingTxn.Gas(),
		To:       pendingTxn.To(),
		Value:    pendingTxn.Value(),
		Data:     pendingTxn.Data(),
	}

	chainID, err := conn.ChainID(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get chain id: %s", err)
	}
	signer := types.NewLondonSigner(chainID)
	txn, err := types.SignTx(types.NewTx(ltxn), signer, pk)
	if err != nil {
		return common.Hash{}, fmt.Errorf("signing txn: %s", err)
	}
	if err := conn.SendTransaction(ctx, txn); err != nil {
		return common.Hash{}, fmt.Errorf("sending txn: %s", err)
	}

	return txn.Hash(), nil
}
