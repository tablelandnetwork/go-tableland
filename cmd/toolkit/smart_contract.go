package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var scCmd = &cobra.Command{
	Use:   "sc",
	Short: "Offers smart sontract calls",
	Long:  `Offers smart contract calls to Tableland Registry`,
	Args:  cobra.ExactArgs(1),
}

var runSQLCmd = &cobra.Command{
	Use:   "sql",
	Short: "Do a RunSQL call to Smart Contract",
	Long:  `Do a RunSQL call to Smart Contract`,
	Args:  cobra.ExactArgs(1),
	PersistentPreRun: func(c *cobra.Command, args []string) {
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		contractAddress, err := cmd.Flags().GetString("contract-address")
		if err != nil {
			return errors.New("failed to parse contract-address")
		}
		chainID, err := cmd.Flags().GetInt("chain-id")
		if err != nil {
			return errors.New("failed to parse chain-id")
		}
		privateKey, err := cmd.Flags().GetString("privatekey")
		if err != nil {
			return errors.New("failed to parse privatekey")
		}
		gatewayEndpoint, err := cmd.Flags().GetString("gateway")
		if err != nil {
			return errors.New("failed to parse gateway")
		}

		ctx := context.Background()
		conn, err := ethclient.Dial(gatewayEndpoint)
		if err != nil {
			return fmt.Errorf("dial: %s", err)
		}

		gasPrice, err := conn.SuggestGasPrice(ctx)
		if err != nil {
			return fmt.Errorf("suggest gas price: %s", err)
		}

		wallet, err := wallet.NewWallet(privateKey)
		if err != nil {
			return fmt.Errorf("new wallet: %s", err)
		}

		auth, err := bind.NewKeyedTransactorWithChainID(wallet.PrivateKey(), big.NewInt(int64(chainID)))
		if err != nil {
			return fmt.Errorf("new keyed transactor with chain id: %s", err)
		}

		nonce, err := conn.PendingNonceAt(ctx, wallet.Address())
		if err != nil {
			return fmt.Errorf("pending nonce at: %s", err)
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
			return fmt.Errorf("new contract: %s", err)
		}

		parser, err := parserimpl.New([]string{
			"sqlite_",
			systemimpl.SystemTablesPrefix,
			systemimpl.RegistryTableName,
		})
		if err != nil {
			return fmt.Errorf("new parser: %s", err)
		}

		query := args[0]
		stmts, err := parser.ValidateMutatingQuery(query, tableland.ChainID(chainID))
		if err != nil {
			return fmt.Errorf("validating mutating query: %s", err)
		}

		tx, err := contract.RunSQL(opts,
			wallet.Address(),
			stmts[0].GetTableID().ToBigInt(),
			query)
		if err != nil {
			return fmt.Errorf("run sql: %s", err)
		}

		fmt.Printf("%s\n\n", tx.Hash())
		return nil
	},
}

var createTableCmd = &cobra.Command{
	Use:   "create",
	Short: "Do a CreateTable call to Smart Contract",
	Long:  `Do a CreateTable call to Smart Contract`,
	Args:  cobra.ExactArgs(1),
	PersistentPreRun: func(c *cobra.Command, args []string) {
	},
	RunE: func(cmd *cobra.Command, args []string) error {
		contractAddress, err := cmd.Flags().GetString("contract-address")
		if err != nil {
			return errors.New("failed to parse contract-address")
		}
		chainID, err := cmd.Flags().GetInt("chain-id")
		if err != nil {
			return errors.New("failed to parse chain-id")
		}
		privateKey, err := cmd.Flags().GetString("privatekey")
		if err != nil {
			return errors.New("failed to parse privatekey")
		}
		gatewayEndpoint, err := cmd.Flags().GetString("gateway")
		if err != nil {
			return errors.New("failed to parse gateway")
		}

		ctx := context.Background()
		conn, err := ethclient.Dial(gatewayEndpoint)
		if err != nil {
			return fmt.Errorf("dial: %s", err)
		}

		gasPrice, err := conn.SuggestGasPrice(ctx)
		if err != nil {
			return fmt.Errorf("suggest gas price: %s", err)
		}

		wallet, err := wallet.NewWallet(privateKey)
		if err != nil {
			return fmt.Errorf("new wallet: %s", err)
		}

		auth, err := bind.NewKeyedTransactorWithChainID(wallet.PrivateKey(), big.NewInt(int64(chainID)))
		if err != nil {
			return fmt.Errorf("new keyed transactor with chain id: %s", err)
		}

		nonce, err := conn.PendingNonceAt(ctx, wallet.Address())
		if err != nil {
			return fmt.Errorf("pending nonce at: %s", err)
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
			return fmt.Errorf("new contract: %s", err)
		}

		parser, err := parserimpl.New([]string{
			"sqlite_",
			systemimpl.SystemTablesPrefix,
			systemimpl.RegistryTableName,
		})
		if err != nil {
			return fmt.Errorf("new parser: %s", err)
		}

		stmt := args[0]
		if _, err := parser.ValidateCreateTable(stmt, tableland.ChainID(chainID)); err != nil {
			return fmt.Errorf("validate create table: %s", err)
		}

		tx, err := contract.CreateTable(opts, wallet.Address(), stmt)
		if err != nil {
			return fmt.Errorf("create table: %s", err)
		}

		fmt.Printf("%s\n\n", tx.Hash())

		return nil
	},
}
