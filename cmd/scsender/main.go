package main

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func main() {
	ctx := context.Background()

	pk := "<fillme>"

	alchemyEndpoint := "wss://opt-kovan.g.alchemy.com/v2/<fillme>"
	chainID := int64(69)
	contractAddress := "<fillme>"

	conn, err := ethclient.Dial(alchemyEndpoint)
	if err != nil {
		panic(err)
	}

	gasPrice, err := conn.SuggestGasPrice(ctx)
	if err != nil {
		panic(err)
	}

	wallet, err := wallet.NewWallet(pk)
	if err != nil {
		panic(err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(wallet.PrivateKey(), big.NewInt(chainID))
	if err != nil {
		panic(err)
	}

	nonce, err := conn.PendingNonceAt(ctx, wallet.Address())
	if err != nil {
		panic(err)
	}
	opts := &bind.TransactOpts{
		Context:  ctx,
		Signer:   auth.Signer,
		From:     auth.From,
		Nonce:    big.NewInt(0).SetUint64(nonce),
		GasPrice: gasPrice,
	}

	contract, err := ethereum.NewContract(common.HexToAddress(contractAddress), conn)
	if err != nil {
		panic(err)
	}

	/*
		tx, err := contract.CreateTable(opts,
			wallet.Address(),
			"CREATE TABLE Healthbot_69 (counter INTEGER);")
		if err != nil {
			panic(err)
		}
	*/
	tx, err := contract.RunSQL(opts,
		wallet.Address(),
		big.NewInt(2),
		"insert into healthbot_69_2 values (1);")
	if err != nil {
		panic(err)
	}

	fmt.Println(tx.Hash())
}
