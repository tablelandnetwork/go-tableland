package client

import (
	"github.com/ethereum/go-ethereum/common"
)

const (
	mainnetURL = "https://tableland.network"
	testnetURL = "https://testnets.tableland.network"
	localURL   = "http://localhost:8080"
)

// ChainID is a supported EVM chain identifier.
type ChainID int64

// ChainIDs is all chain ids supported by Tableland.
var ChainIDs = struct {
	Ethereum           ChainID
	Optimism           ChainID
	Polygon            ChainID
	Arbitrum           ChainID
	ArbitrumNova       ChainID
	Filecoin           ChainID
	EthereumGoerli     ChainID
	OptimismGoerli     ChainID
	ArbitrumGoerli     ChainID
	FilecoinHyperspace ChainID
	PolygonMumbai      ChainID
	Local              ChainID
}{
	Ethereum:           1,
	Optimism:           10,
	Polygon:            137,
	Arbitrum:           42161,
	ArbitrumNova:   42170,
	Filecoin:           314,
	EthereumGoerli:     5,
	OptimismGoerli:     420,
	ArbitrumGoerli:     421613,
	FilecoinHyperspace: 3141,
	PolygonMumbai:      80001,
	Local:              31337,
}

// Chain is a info about a network supported by Talbleland.
type Chain struct {
	Endpoint     string
	ID           ChainID
	Name         string
	ContractAddr common.Address
}

// Chains is the connection info for all chains supported by Tableland.
var Chains = map[ChainID]Chain{
	ChainIDs.Ethereum: {
		Endpoint:     mainnetURL,
		ID:           ChainIDs.Ethereum,
		Name:         "Ethereum",
		ContractAddr: common.HexToAddress("0x012969f7e3439a9B04025b5a049EB9BAD82A8C12"),
	},
	ChainIDs.Optimism: {
		Endpoint:     mainnetURL,
		ID:           ChainIDs.Optimism,
		Name:         "Optimism",
		ContractAddr: common.HexToAddress("0xfad44BF5B843dE943a09D4f3E84949A11d3aa3e6"),
	},
	ChainIDs.Polygon: {
		Endpoint:     mainnetURL,
		ID:           ChainIDs.Polygon,
		Name:         "Polygon",
		ContractAddr: common.HexToAddress("0x5c4e6A9e5C1e1BF445A062006faF19EA6c49aFeA"),
	},
	ChainIDs.Arbitrum: {
		Endpoint:     mainnetURL,
		ID:           ChainIDs.Arbitrum,
		Name:         "Arbitrum",
		ContractAddr: common.HexToAddress("0x9aBd75E8640871A5a20d3B4eE6330a04c962aFfd"),
	},
	ChainIDs.EthereumGoerli: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.EthereumGoerli,
		Name:         "Ethereum Goerli",
		ContractAddr: common.HexToAddress("0xDA8EA22d092307874f30A1F277D1388dca0BA97a"),
	},
	ChainIDs.OptimismGoerli: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.OptimismGoerli,
		Name:         "Optimism Goerli",
		ContractAddr: common.HexToAddress("0xC72E8a7Be04f2469f8C2dB3F1BdF69A7D516aBbA"),
	},
	ChainIDs.ArbitrumGoerli: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.ArbitrumGoerli,
		Name:         "Arbitrum Goerli",
		ContractAddr: common.HexToAddress("0x033f69e8d119205089Ab15D340F5b797732f646b"),
	},
	ChainIDs.FilecoinHyperspace: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.FilecoinHyperspace,
		Name:         "Filecoin Hyperspace",
		ContractAddr: common.HexToAddress("0x86FB37A952463f3Ca5D18214300276cF3e3FeaA1"),
	},
	ChainIDs.PolygonMumbai: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.PolygonMumbai,
		Name:         "Polygon Mumbai",
		ContractAddr: common.HexToAddress("0x4b48841d4b32C4650E4ABc117A03FE8B51f38F68"),
	},
	ChainIDs.Local: {
		Endpoint:     localURL,
		ID:           ChainIDs.Local,
		Name:         "Local",
		ContractAddr: common.HexToAddress("0xe7f1725e7734ce288f8367e1bb143e90bb3f0512"),
	},
}

// InfuraURLs contains the URLs for supported chains for Infura.
var InfuraURLs = map[ChainID]string{
	ChainIDs.EthereumGoerli: "https://goerli.infura.io/v3/%s",
	ChainIDs.Ethereum:       "https://mainnet.infura.io/v3/%s",
	ChainIDs.OptimismGoerli: "https://optimism-goerli.infura.io/v3/%s",
	ChainIDs.Optimism:       "https://optimism-mainnet.infura.io/v3/%s",
	ChainIDs.ArbitrumGoerli: "https://arbitrum-goerli.infura.io/v3/%s",
	ChainIDs.Arbitrum:       "https://arbitrum-mainnet.infura.io/v3/%s",
	ChainIDs.PolygonMumbai:  "https://polygon-mumbai.infura.io/v3/%s",
	ChainIDs.Polygon:        "https://polygon-mainnet.infura.io/v3/%s",
}

// AlchemyURLs contains the URLs for supported chains for Alchemy.
var AlchemyURLs = map[ChainID]string{
	ChainIDs.EthereumGoerli: "https://eth-goerli.g.alchemy.com/v2/%s",
	ChainIDs.Ethereum:       "https://eth-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.OptimismGoerli: "https://opt-goerli.g.alchemy.com/v2/%s",
	ChainIDs.Optimism:       "https://opt-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.ArbitrumGoerli: "https://arb-goerli.g.alchemy.com/v2/%s",
	ChainIDs.Arbitrum:       "https://arb-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.PolygonMumbai:  "https://polygon-mumbai.g.alchemy.com/v2/%s",
	ChainIDs.Polygon:        "https://polygon-mainnet.g.alchemy.com/v2/%s",
}

// QuickNodeURLs contains the URLs for supported chains for QuickNode.
var QuickNodeURLs = map[ChainID]string{
	ChainIDs.ArbitrumNova: "https://skilled-yolo-mountain.nova-mainnet.discover.quiknode.pro/%s",
}

// LocalURLs contains the URLs for a local network.
var LocalURLs = map[ChainID]string{
	ChainIDs.Local: "http://localhost:8545",
}

// AnkrURLs contains the URLs for supported chains on Ankr.
var AnkrURLs = map[ChainID]string{
	ChainIDs.FilecoinHyperspace: "https://rpc.ankr.com/filecoin_testnet/%s",
	ChainIDs.Filecoin:           "https://rpc.ankr.com/filecoin/%s",
}
