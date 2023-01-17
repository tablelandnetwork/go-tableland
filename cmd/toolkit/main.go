package main

import (
	"time"

	"github.com/spf13/cobra"
)

var cliName = "toolkit"

var rootCmd = &cobra.Command{
	Use:   cliName,
	Short: "toolkit is CLI for Tableland developers",
	Long:  `toolkit is CLI for Tableland developers executing mundane tasks`,
	Args:  cobra.ExactArgs(0),
}

func main() {
	rootCmd.Execute() //nolint
}

func init() {
	rootCmd.AddCommand(siweCmd)
	rootCmd.AddCommand(scCmd)
	rootCmd.AddCommand(walletCmd)
	rootCmd.AddCommand(gasPriceBumperCmd)

	siweCreateCmd.Flags().Duration("duration", time.Hour*24*365*100, "validity duration")
	siweCreateCmd.Flags().Int("chain-id", 69, "chain id")
	siweCmd.AddCommand(siweCreateCmd)

	scCmd.PersistentFlags().String("contract-address", "", "the smart contract address")
	scCmd.PersistentFlags().Int("chain-id", 69, "chain id")
	scCmd.PersistentFlags().String("privatekey", "", "the private key used to make the contract calls")
	scCmd.PersistentFlags().String("gateway", "", "URL of an Ethereum node API (i.e: Alchemy/Infura)")
	scCmd.AddCommand(runSQLCmd)
	scCmd.AddCommand(createTableCmd)
	scCmd.AddCommand(setControllerCmd)

	walletCreateCmd.Flags().String("filename", "privatekey.hex", "Filename to store hex representation of private key")
	walletCmd.AddCommand(walletCreateCmd)

	gasPriceBumperCmd.PersistentFlags().String("privatekey", "", "the private key used to make the contract calls")
	gasPriceBumperCmd.PersistentFlags().String("gateway", "", "URL of an Ethereum node API (i.e: Alchemy/Infura)")
}
