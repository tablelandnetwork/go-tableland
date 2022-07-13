package main

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/siwe"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var siweCmd = &cobra.Command{
	Use:   "siwe",
	Short: "SIWE utilities",
	Long:  `Sign-In With Ethereum utilities`,
	Args:  cobra.ExactArgs(1),
}

var siweCreateCmd = &cobra.Command{
	Use:   "create",
	Short: "Creates a SIWE token",
	Long:  `Creates a SIWE token to be used in Tableland RPC calls`,
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		duration, err := cmd.Flags().GetDuration("duration")
		if err != nil {
			return errors.New("failed to parse duration")
		}
		chainID, err := cmd.Flags().GetInt("chain-id")
		if err != nil {
			return errors.New("failed to parse chain-id")
		}
		privateKey := args[0]

		w, err := wallet.NewWallet(privateKey)
		if err != nil {
			return fmt.Errorf("decoding private key: %s", err)
		}

		siwe, err := siwe.EncodedSIWEMsg(tableland.ChainID(chainID), w, duration)
		if err != nil {
			return fmt.Errorf("creating bearer value: %v", err)
		}

		fmt.Printf("%s\n\n", siwe)
		fmt.Printf("Signed by %s\n", w.Address().Hex())

		return nil
	},
}
