package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
)

var gasPriceBumperCmd = &cobra.Command{
	Use:   "gaspricebump",
	Short: "Bumps gas price for a stuck transaction",
	Long:  `Bumps gas price for a stuck transaction`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		privateKey, err := cmd.Flags().GetString("privatekey")
		if err != nil {
			return errors.New("failed to parse privatekey")
		}
		gatewayEndpoint, err := cmd.Flags().GetString("gateway")
		if err != nil {
			return errors.New("failed to parse gateway")
		}

		stuckTxnHash := common.HexToHash(args[0])
		pk, err := crypto.HexToECDSA(privateKey)
		if err != nil {
			log.Fatalf("decoding private key: %s", err)
		}

		conn, err := ethclient.Dial(gatewayEndpoint)
		if err != nil {
			log.Fatalf("failed to connect to ethereum endpoint: %s", err)
		}

		newTxnHash, err := bumpTxnFee(conn, pk, stuckTxnHash)
		if err != nil {
			log.Fatalf("bumpint txn fee: %s", err)
		}
		fmt.Printf("The new transaction hash is: %s\n", newTxnHash)

		return nil
	},
}

func bumpTxnFee(
	conn *ethclient.Client,
	pk *ecdsa.PrivateKey,
	stuckTxnHash common.Hash,
) (common.Hash, error) {
	ctx := context.Background()

	pendingTxn, isPending, err := conn.TransactionByHash(ctx, stuckTxnHash)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get pending txn from the mempool: %s", err)
	}
	if !isPending {
		return common.Hash{}, fmt.Errorf("the transaction hash %s isn't pending", stuckTxnHash)
	}

	candidateGasPriceSuggested, err := conn.SuggestGasPrice(ctx)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get suggested gas price: %s", err)
	}
	candidateOldGasPricePlus25 := big.NewInt(0).
		Div(big.NewInt(0).Mul(pendingTxn.GasPrice(), big.NewInt(125)), big.NewInt(100))

	newGasPrice := candidateOldGasPricePlus25
	if newGasPrice.Cmp(candidateGasPriceSuggested) < 0 {
		newGasPrice = candidateGasPriceSuggested
	}

	fmt.Printf("Current txn gas price: %s\n", pendingTxn.GasPrice())
	fmt.Printf("Candidate prices, +25%%: %s, Suggested: %s\n\n", candidateOldGasPricePlus25, candidateGasPriceSuggested)
	fmt.Printf("**New gas price: %s**\n", newGasPrice)

	ltxn := &types.LegacyTx{
		Nonce:    pendingTxn.Nonce(),
		GasPrice: newGasPrice,
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
