package client

import (
	"log"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/contract"
)

func main() {
	// Create an IPC based RPC connection to a remote node
	conn, err := ethclient.Dial("/home/karalabe/.ethereum/testnet/geth.ipc")
	if err != nil {
		log.Fatalf("Failed to connect to the Ethereum client: %v", err)
	}
	// Instantiate the contract and display its name
	_, err = contract.NewContract(common.HexToAddress("0x21e6fc92f93c8a1bb41e2be64b4e1f88a54d3576"), conn)
	if err != nil {
		log.Fatalf("Failed to instantiate a Token contract: %v", err)
	}
	// name, err := token.Mint()
	// if err != nil {
	// 	log.Fatalf("Failed to retrieve token name: %v", err)
	// }
	// fmt.Println("Token name:", name)
}
