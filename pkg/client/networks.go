package client

import (
	"github.com/ethereum/go-ethereum/common"
)

// Network represents a Tableland network.
type Network string

const (
	// Testnet is the Tableland testnet network.
	Testnet Network = "https://testnet.tableland.network"
	// Staging is the Tableland staging network.
	Staging Network = "https://staging.tableland.network"
	// Localhost is the Tableland local network.
	Localhost Network = "http://localhost:8080"
)

// ChainID is a supported EVM chain identifier.
type ChainID int64

const (
	// EthereumGoerli is the Ethereum Goerli chain id.
	EthereumGoerli ChainID = 5
	// Ethereum is the Ethereum chain id.
	Ethereum ChainID = 1
	// OptimismKovan is the Optimism Kovan chains id.
	OptimismKovan ChainID = 69
	// OptimismGoerli is the Optimism Goerli chain id.
	OptimismGoerli ChainID = 420
	// Optimism is the Optmism chain id.
	Optimism ChainID = 10
	// ArbitrumGoerli is the Arbitrum Goerli chain id.
	ArbitrumGoerli ChainID = 421613
	// PolygonMumbai is the Poygon Mumbai chain id.
	PolygonMumbai ChainID = 80001
	// Polygon is the Polygon chiain id.
	Polygon ChainID = 137
	// Local is the typical local chain id.
	Local ChainID = 31337
)

var infuraURLs = map[ChainID]string{
	EthereumGoerli: "https://goerli.infura.io/v3/%s",
	Ethereum:       "https://mainnet.infura.io/v3/%s",
	OptimismKovan:  "https://optimism-kovan.infura.io/v3/%s",
	OptimismGoerli: "https://optimism-goerli.infura.io/v3/%s",
	Optimism:       "https://optimism-mainnet.infura.io/v3/%s",
	ArbitrumGoerli: "https://arbitrim-goerli.infura.io/v3/%s", // TODO: Check this, requires upgrade.
	PolygonMumbai:  "https://polygon-mumbai.infura.io/v3/%s",
	Polygon:        "https://polygon-mainnet.infura.io/v3/%s",
}

var alchemyURLs = map[ChainID]string{
	EthereumGoerli: "https://eth-goerli.g.alchemy.com/v2/%s",
	Ethereum:       "https://eth-mainnet.g.alchemy.com/v2/%s",
	OptimismKovan:  "https://opt-kovan.g.alchemy.com/v2/%s",
	OptimismGoerli: "https://opt-goerli.g.alchemy.com/v2/%s",
	Optimism:       "https://opt-mainnet.g.alchemy.com/v2/%s",
	ArbitrumGoerli: "https://arb-goerli.g.alchemy.com/v2/%s",
	PolygonMumbai:  "https://polygon-mumbai.g.alchemy.com/v2/%s",
	Polygon:        "https://polygon-mainnet.g.alchemy.com/v2/%s",
}

var localURLs = map[ChainID]string{
	Local: "http://localhost:8545",
}

// NetworkInfo is a info about a network supported by Talbleland.
type NetworkInfo struct {
	Network      Network
	ChainID      ChainID
	ContractAddr common.Address
}

var (
	// Tableland testnet mainnets.
	//--------------------------------------------------------------------------------------.

	// TestnetEtherum is network: Testnet, chain: Ethereum.
	TestnetEtherum = NetworkInfo{
		Network:      Testnet,
		ChainID:      Ethereum,
		ContractAddr: common.HexToAddress("0x012969f7e3439a9B04025b5a049EB9BAD82A8C12"),
	}
	// TestnetOptimism is network: Testnet, chain: Optimism.
	TestnetOptimism = NetworkInfo{
		Network:      Testnet,
		ChainID:      Optimism,
		ContractAddr: common.HexToAddress("0xfad44BF5B843dE943a09D4f3E84949A11d3aa3e6"),
	}
	// TestnetPolygon is network: Testnet, chain: Polygon.
	TestnetPolygon = NetworkInfo{
		Network:      Testnet,
		ChainID:      Polygon,
		ContractAddr: common.HexToAddress("0x5c4e6A9e5C1e1BF445A062006faF19EA6c49aFeA"),
	}

	// Tableland testnet testnets.
	//--------------------------------------------------------------------------------------.

	// TestnetEthereumGoerli is network: Testnet, chain: EthereumGoerli.
	TestnetEthereumGoerli = NetworkInfo{
		Network:      Testnet,
		ChainID:      EthereumGoerli,
		ContractAddr: common.HexToAddress("0xDA8EA22d092307874f30A1F277D1388dca0BA97a"),
	}
	// TestnetOptimismKovan is network: Testnet, chain: OptimismKovan.
	TestnetOptimismKovan = NetworkInfo{
		Network:      Testnet,
		ChainID:      OptimismKovan,
		ContractAddr: common.HexToAddress("0xf2C9Fc73884A9c6e6Db58778176Ab67989139D06"),
	}
	// TestnetOptimismGoerli is network: Testnet, chain: OptimismGoerli.
	TestnetOptimismGoerli = NetworkInfo{
		Network:      Testnet,
		ChainID:      OptimismGoerli,
		ContractAddr: common.HexToAddress("0xC72E8a7Be04f2469f8C2dB3F1BdF69A7D516aBbA"),
	}
	// TestnetArbitrumGoerli is network: Testnet, chain: ArbitrumGoerli.
	TestnetArbitrumGoerli = NetworkInfo{
		Network:      Testnet,
		ChainID:      ArbitrumGoerli,
		ContractAddr: common.HexToAddress("0x033f69e8d119205089Ab15D340F5b797732f646b"),
	}
	// TestnetPolygonMumbai is network: Testnet, chain: PolygonMumbai.
	TestnetPolygonMumbai = NetworkInfo{
		Network:      Testnet,
		ChainID:      PolygonMumbai,
		ContractAddr: common.HexToAddress("0x4b48841d4b32C4650E4ABc117A03FE8B51f38F68"),
	}

	// Tableland staging testnets.
	//--------------------------------------------------------------------------------------.

	// StagingOptimismKovan is network: Staging, chain: OptimismKovan.
	StagingOptimismKovan = NetworkInfo{
		Network:      Staging,
		ChainID:      OptimismKovan,
		ContractAddr: common.HexToAddress("0x7E57BaA6724c7742de6843094002c4e58FF6c7c3"),
	}
	// StagingOptimismGoerli is network: Staging, chain: OptimismGoerli.
	StagingOptimismGoerli = NetworkInfo{
		Network:      Staging,
		ChainID:      OptimismGoerli,
		ContractAddr: common.HexToAddress("0xfe79824f6E5894a3DD86908e637B7B4AF57eEE28"),
	}

	// Local
	//--------------------------------------------------------------------------------------.

	// LocalhostLocal is network: LocalDev, chain: Local.
	LocalhostLocal = NetworkInfo{
		Network:      Localhost,
		ChainID:      Local,
		ContractAddr: common.HexToAddress("0xe7f1725e7734ce288f8367e1bb143e90bb3f0512"),
	}
)

// CanRelayWrites returns whether Tableland validators will relay write requests.
func (sn NetworkInfo) CanRelayWrites() bool {
	return sn.ChainID != Ethereum && sn.ChainID != Optimism && sn.ChainID != Polygon
}
