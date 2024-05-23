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
	Ethereum            ChainID
	Optimism            ChainID
	Polygon             ChainID
	Arbitrum            ChainID
	ArbitrumNova        ChainID
	Filecoin            ChainID
	EthereumSepolia     ChainID
	OptimismSepolia     ChainID
	ArbitrumSepolia     ChainID
	BaseSepolia         ChainID
	FilecoinCalibration ChainID
	PolygonAmoy         ChainID
	Local               ChainID
}{
	Ethereum:            1,
	Optimism:            10,
	Polygon:             137,
	Arbitrum:            42161,
	ArbitrumNova:        42170,
	Filecoin:            314,
	EthereumSepolia:     11155111,
	OptimismSepolia:     11155420,
	ArbitrumSepolia:     421614,
	BaseSepolia:         84532,
	FilecoinCalibration: 314159,
	PolygonAmoy:         80002,
	Local:               31337,
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
	ChainIDs.ArbitrumNova: {
		Endpoint:     mainnetURL,
		ID:           ChainIDs.ArbitrumNova,
		Name:         "Arbitrum Nova",
		ContractAddr: common.HexToAddress("0x1a22854c5b1642760a827f20137a67930ae108d2"),
	},
	ChainIDs.Filecoin: {
		Endpoint:     mainnetURL,
		ID:           ChainIDs.Filecoin,
		Name:         "Filecoin",
		ContractAddr: common.HexToAddress("0x59EF8Bf2d6c102B4c42AEf9189e1a9F0ABfD652d"),
	},
	ChainIDs.EthereumSepolia: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.EthereumSepolia,
		Name:         "Ethereum Sepolia",
		ContractAddr: common.HexToAddress("0xc50C62498448ACc8dBdE43DA77f8D5D2E2c7597D"),
	},
	ChainIDs.OptimismSepolia: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.OptimismSepolia,
		Name:         "Optimism Sepolia",
		ContractAddr: common.HexToAddress("0x68A2f4423ad3bf5139Db563CF3bC80aA09ed7079"),
	},
	ChainIDs.ArbitrumSepolia: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.ArbitrumSepolia,
		Name:         "Arbitrum Sepolia",
		ContractAddr: common.HexToAddress("0x223A74B8323914afDC3ff1e5005564dC17231d6e"),
	},
	ChainIDs.BaseSepolia: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.BaseSepolia,
		Name:         "Base Sepolia",
		ContractAddr: common.HexToAddress("0xA85aAE9f0Aec5F5638E5F13840797303Ab29c9f9"),
	},
	ChainIDs.FilecoinCalibration: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.FilecoinCalibration,
		Name:         "Filecoin Calibration",
		ContractAddr: common.HexToAddress("0x030BCf3D50cad04c2e57391B12740982A9308621"),
	},
	ChainIDs.PolygonAmoy: {
		Endpoint:     testnetURL,
		ID:           ChainIDs.PolygonAmoy,
		Name:         "Polygon Amoy",
		ContractAddr: common.HexToAddress("0x170fb206132b693e38adFc8727dCfa303546Cec1"),
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
	ChainIDs.Ethereum: "https://mainnet.infura.io/v3/%s",
	ChainIDs.Optimism: "https://optimism-mainnet.infura.io/v3/%s",
	ChainIDs.Arbitrum: "https://arbitrum-mainnet.infura.io/v3/%s",
	ChainIDs.Polygon:  "https://polygon-mainnet.infura.io/v3/%s",
}

// AlchemyURLs contains the URLs for supported chains for Alchemy.
var AlchemyURLs = map[ChainID]string{
	ChainIDs.EthereumSepolia: "https://eth-sepolia.g.alchemy.com/v2/%s",
	ChainIDs.Ethereum:        "https://eth-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.OptimismSepolia: "https://opt-sepolia.g.alchemy.com/v2/%s",
	ChainIDs.Optimism:        "https://opt-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.ArbitrumSepolia: "https://arb-sepolia.g.alchemy.com/v2/%s",
	ChainIDs.Arbitrum:        "https://arb-mainnet.g.alchemy.com/v2/%s",
	ChainIDs.BaseSepolia:     "https://base-sepolia.g.alchemy.com/v2/%s",
	ChainIDs.PolygonAmoy:     "https://polygon-amoy.g.alchemy.com/v2/%s",
	ChainIDs.Polygon:         "https://polygon-mainnet.g.alchemy.com/v2/%s",
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
	ChainIDs.Filecoin: "https://rpc.ankr.com/filecoin/%s",
}

// GlifURLs contains the URLs for supported chains on Glif.
var GlifURLs = map[ChainID]string{
	ChainIDs.FilecoinCalibration: "https://api.calibration.node.glif.io/rpc/v1%s",
	ChainIDs.Filecoin:            "https://api.node.glif.io/rpc/v1%s",
}
