package client

import (
	"github.com/ethereum/go-ethereum/common"
)

const (
	testnetURL = "https://testnet.tableland.network"
	localURL   = "http://localhost:8080"
)

// ChainID is a supported EVM chain identifier.
type ChainID int64

// ChainIDs is all chain ids supported by Tableland.
var ChainIDs = struct {
	Ethereum       ChainID
	Optimism       ChainID
	Polygon        ChainID
	EthereumGoerli ChainID
	OptimismGoerli ChainID
	ArbitrumGoerli ChainID
	Arbitrum       ChainID
	PolygonMumbai  ChainID
	Local          ChainID
}{
	Ethereum:       1,
	Optimism:       10,
	Polygon:        137,
	EthereumGoerli: 5,
	OptimismGoerli: 420,
	ArbitrumGoerli: 421613,
	Arbitrum:       42161,
	PolygonMumbai:  80001,
	Local:          31337,
}

// Chain is a info about a network supported by Talbleland.
type Chain struct {
	Endpoint     string
	ID           ChainID
	ContractAddr common.Address
}

// Chains is the connection info for all chains supported by Tableland.
var Chains = struct {
	Ethereum       Chain
	Optimism       Chain
	Polygon        Chain
	Arbitrum       Chain
	EthereumGoerli Chain
	OptimismGoerli Chain
	ArbitrumGoerli Chain
	PolygonMumbai  Chain
	Local          Chain
}{
	Ethereum: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.Ethereum,
		ContractAddr: common.HexToAddress("0x012969f7e3439a9B04025b5a049EB9BAD82A8C12"),
	},
	Optimism: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.Optimism,
		ContractAddr: common.HexToAddress("0xfad44BF5B843dE943a09D4f3E84949A11d3aa3e6"),
	},
	Polygon: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.Polygon,
		ContractAddr: common.HexToAddress("0x5c4e6A9e5C1e1BF445A062006faF19EA6c49aFeA"),
	},
	EthereumGoerli: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.EthereumGoerli,
		ContractAddr: common.HexToAddress("0xDA8EA22d092307874f30A1F277D1388dca0BA97a"),
	},
	OptimismGoerli: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.OptimismGoerli,
		ContractAddr: common.HexToAddress("0xC72E8a7Be04f2469f8C2dB3F1BdF69A7D516aBbA"),
	},
	ArbitrumGoerli: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.ArbitrumGoerli,
		ContractAddr: common.HexToAddress("0x033f69e8d119205089Ab15D340F5b797732f646b"),
	},
	Arbitrum: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.Arbitrum,
		ContractAddr: common.HexToAddress("TBD"),
	},
	PolygonMumbai: Chain{
		Endpoint:     testnetURL,
		ID:           ChainIDs.PolygonMumbai,
		ContractAddr: common.HexToAddress("0x4b48841d4b32C4650E4ABc117A03FE8B51f38F68"),
	},
	Local: Chain{
		Endpoint:     localURL,
		ID:           ChainIDs.Local,
		ContractAddr: common.HexToAddress("0xe7f1725e7734ce288f8367e1bb143e90bb3f0512"),
	},
}

// CanRelayWrites returns whether Tableland validators will relay write requests.
func (c Chain) CanRelayWrites() bool {
	return c.ID != ChainIDs.Ethereum && c.ID != ChainIDs.Optimism && c.ID != ChainIDs.Polygon
}

var infuraURLs = map[ChainID]string{
	ChainIDs.EthereumGoerli: "https://goerli.infura.io/v3/%s",
	ChainIDs.Ethereum:       "https://mainnet.infura.io/v3/%s",
	ChainIDs.OptimismGoerli: "https://optimism-goerli.infura.io/v3/%s",
	ChainIDs.Optimism:       "https://optimism-mainnet.infura.io/v3/%s",
	ChainIDs.ArbitrumGoerli: "https://arbitrum-goerli.infura.io/v3/%s",
	ChainIDs.Arbitrum:       "https://arbitrum-mainnet.infura.io/v3/%s",
	ChainIDs.PolygonMumbai:  "https://polygon-mumbai.infura.io/v3/%s",
	ChainIDs.Polygon:        "https://polygon-mainnet.infura.io/v3/%s",
}

var alchemyURLs = map[ChainID]string{
	ChainIDs.EthereumGoerli: "https://eth-goerli.g.alchemy.com/v2/%s",
	ChainIDs.Ethereum:       "https://eth-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.OptimismGoerli: "https://opt-goerli.g.alchemy.com/v2/%s",
	ChainIDs.Optimism:       "https://opt-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.ArbitrumGoerli: "https://arb-goerli.g.alchemy.com/v2/%s",
	ChainIDs.Arbitrum:       "https://arb-goerli.g.alchemy.com/v2/%s",
	ChainIDs.PolygonMumbai:  "https://polygon-mumbai.g.alchemy.com/v2/%s",
	ChainIDs.Polygon:        "https://polygon-mainnet.g.alchemy.com/v2/%s",
}

var localURLs = map[ChainID]string{
	ChainIDs.Local: "http://localhost:8545",
}
