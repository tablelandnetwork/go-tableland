package main

import (
	"crypto/ecdsa"
	"errors"
	"fmt"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spf13/cobra"
)

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Offers wallet utilites",
	Long:  `Offers wallet utilites`,
	Args:  cobra.ExactArgs(1),
}

var walletCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates an ETH wallet",
	Long:  `Creates an ETH wallet`,
	Args:  cobra.ExactArgs(0),
	RunE: func(cmd *cobra.Command, args []string) error {
		filename, err := cmd.Flags().GetString("filename")
		if err != nil {
			return errors.New("failed to parse filename")
		}
		privateKey, err := crypto.GenerateKey()
		if err != nil {
			return fmt.Errorf("generate key: %s", err)
		}
		privateKeyBytes := crypto.FromECDSA(privateKey)

		if err := os.WriteFile(filename, []byte(hexutil.Encode(privateKeyBytes)[2:]), 0o644); err != nil {
			return fmt.Errorf("writing to file %s: %s", filename, err)
		}
		pubk, _ := privateKey.Public().(*ecdsa.PublicKey)
		publicKey := common.HexToAddress(crypto.PubkeyToAddress(*pubk).Hex())

		fmt.Printf("Wallet address %s created\n", publicKey)
		fmt.Printf("Private key saved in %s\n", filename)

		return nil
	},
}

var walletAddressCmd = &cobra.Command{
	Use:   "address",
	Short: "Returns address of ETH wallet",
	Long:  `Returns address of ETH wallet`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		privateKey, err := crypto.HexToECDSA(args[0])
		if err != nil {
			return fmt.Errorf("decode key: %s", err)
		}

		pubk, _ := privateKey.Public().(*ecdsa.PublicKey)
		publicKey := common.HexToAddress(crypto.PubkeyToAddress(*pubk).Hex())

		fmt.Printf("Wallet address %s\n", publicKey)

		return nil
	},
}
