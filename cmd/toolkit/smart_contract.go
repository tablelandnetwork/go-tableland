package main

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/spf13/cobra"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tables"
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

		wallet, err := wallet.NewWallet(privateKey)
		if err != nil {
			return fmt.Errorf("new wallet: %s", err)
		}

		parser, err := parserimpl.New([]string{
			"sqlite_",
			parsing.SystemTablesPrefix,
			parsing.RegistryTableName,
		})
		if err != nil {
			return fmt.Errorf("new parser: %s", err)
		}

		query := args[0]
		stmts, err := parser.ValidateMutatingQuery(query, tableland.ChainID(chainID))
		if err != nil {
			return fmt.Errorf("validating mutating query: %s", err)
		}

		client, err := ethereum.NewClient(
			conn,
			tableland.ChainID(chainID),
			common.HexToAddress(contractAddress),
			wallet,
			impl.NewSimpleTracker(wallet, conn),
		)
		if err != nil {
			return fmt.Errorf("creating ethereum client: %s", err)
		}

		tx, err := client.RunSQL(ctx,
			wallet.Address(),
			stmts[0].GetTableID(),
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

		wallet, err := wallet.NewWallet(privateKey)
		if err != nil {
			return fmt.Errorf("new wallet: %s", err)
		}

		parser, err := parserimpl.New([]string{
			"sqlite_",
			parsing.SystemTablesPrefix,
			parsing.RegistryTableName,
		})
		if err != nil {
			return fmt.Errorf("new parser: %s", err)
		}

		stmt := args[0]
		if _, err := parser.ValidateCreateTable(stmt, tableland.ChainID(chainID)); err != nil {
			return fmt.Errorf("validate create table: %s", err)
		}

		client, err := ethereum.NewClient(
			conn,
			tableland.ChainID(chainID),
			common.HexToAddress(contractAddress),
			wallet,
			impl.NewSimpleTracker(wallet, conn),
		)
		if err != nil {
			return fmt.Errorf("creating ethereum client: %s", err)
		}

		tx, err := client.CreateTable(ctx, wallet.Address(), stmt)
		if err != nil {
			return fmt.Errorf("create table: %s", err)
		}

		fmt.Printf("%s\n\n", tx.Hash())

		return nil
	},
}

var setControllerCmd = &cobra.Command{
	Use:   "setcontroller",
	Short: "Do a SetController call to Smart Contract",
	Long:  `Do a SetController call to Smart Contract`,
	Args:  cobra.ExactArgs(2),
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

		wallet, err := wallet.NewWallet(privateKey)
		if err != nil {
			return fmt.Errorf("new wallet: %s", err)
		}

		tableIDStr := args[0]
		controller := args[1]

		table := new(big.Int)
		tableID, ok := table.SetString(tableIDStr, 10)
		if !ok {
			return fmt.Errorf("set string: %s", err)
		}

		client, err := ethereum.NewClient(
			conn,
			tableland.ChainID(chainID),
			common.HexToAddress(contractAddress),
			wallet,
			impl.NewSimpleTracker(wallet, conn),
		)
		if err != nil {
			return fmt.Errorf("creating ethereum client: %s", err)
		}

		tx, err := client.SetController(
			ctx,
			wallet.Address(),
			tables.TableID(*tableID),
			common.HexToAddress(controller),
		)
		if err != nil {
			return fmt.Errorf("set controller: %s", err)
		}

		fmt.Printf("%s\n\n", tx.Hash())

		return nil
	},
}
