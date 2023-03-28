package v1

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var defaultChain = client.Chains[client.ChainIDs.PolygonMumbai]

// Client is the Tableland client.
type Client struct {
	tblHTTP     *http.Client
	tblContract *ethereum.Client
	chain       client.Chain
	wallet      *wallet.Wallet
	parser      parsing.SQLValidator
	baseURL     *url.URL
}

type config struct {
	chain           *client.Chain
	infuraAPIKey    string
	alchemyAPIKey   string
	local           bool
	contractBackend bind.ContractBackend
}

// NewClientOption controls the behavior of NewClient.
type NewClientOption func(*config)

// NewClientChain specifies chaininfo.
func NewClientChain(chain client.Chain) NewClientOption {
	return func(ncc *config) {
		ncc.chain = &chain
	}
}

// NewClientInfuraAPIKey specifies an Infura API to use when creating an EVM backend.
func NewClientInfuraAPIKey(key string) NewClientOption {
	return func(c *config) {
		c.infuraAPIKey = key
	}
}

// NewClientAlchemyAPIKey specifies an Alchemy API to use when creating an EVM backend.
func NewClientAlchemyAPIKey(key string) NewClientOption {
	return func(c *config) {
		c.alchemyAPIKey = key
	}
}

// NewClientLocal specifies that a local EVM backend should be used.
func NewClientLocal() NewClientOption {
	return func(c *config) {
		c.local = true
	}
}

// NewClientContractBackend specifies a custom EVM backend to use.
func NewClientContractBackend(backend bind.ContractBackend) NewClientOption {
	return func(c *config) {
		c.contractBackend = backend
	}
}

// NewClient creates a new Client.
func NewClient(ctx context.Context, wallet *wallet.Wallet, opts ...NewClientOption) (*Client, error) {
	config := config{chain: &defaultChain}
	for _, opt := range opts {
		opt(&config)
	}

	contractBackend, err := getContractBackend(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("getting contract backend: %v", err)
	}

	tblContract, err := ethereum.NewClient(
		contractBackend,
		tableland.ChainID(config.chain.ID),
		config.chain.ContractAddr,
		wallet,
		impl.NewSimpleTracker(wallet, contractBackend),
	)
	if err != nil {
		return nil, fmt.Errorf("creating contract client: %v", err)
	}

	parserOpts := []parsing.Option{
		parsing.WithMaxReadQuerySize(35000),
		parsing.WithMaxWriteQuerySize(35000),
	}

	parser, err := parserimpl.New([]string{
		"sqlite_",
		parsing.SystemTablesPrefix,
		parsing.RegistryTableName,
	}, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("new parser: %s", err)
	}

	baseURL, err := url.Parse(config.chain.Endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid endpoint URL: %s", err)
	}

	return &Client{
		tblHTTP: &http.Client{
			Timeout: time.Second * 30,
		},
		tblContract: tblContract,
		chain:       *config.chain,
		wallet:      wallet,
		parser:      parser,
		baseURL:     baseURL,
	}, nil
}

func getContractBackend(ctx context.Context, config config) (bind.ContractBackend, error) {
	if config.contractBackend != nil && config.infuraAPIKey == "" && config.alchemyAPIKey == "" {
		return config.contractBackend, nil
	} else if config.infuraAPIKey != "" && config.contractBackend == nil && config.alchemyAPIKey == "" {
		tmpl, found := client.InfuraURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Infura", config.chain.ID)
		}
		return ethclient.DialContext(ctx, fmt.Sprintf(tmpl, config.infuraAPIKey))
	} else if config.alchemyAPIKey != "" && config.contractBackend == nil && config.infuraAPIKey == "" {
		tmpl, found := client.AlchemyURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Alchemy", config.chain.ID)
		}
		return ethclient.DialContext(ctx, fmt.Sprintf(tmpl, config.alchemyAPIKey))
	} else if config.local {
		url, found := client.LocalURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Local", config.chain.ID)
		}
		return ethclient.DialContext(ctx, url)
	}
	return nil, errors.New("no provider specified, must provide an Infura API key, Alchemy API key, or an ETH backend")
}

// TableID is the ID of a Table.
type TableID big.Int

// String returns a string representation of the TableID.
func (tid TableID) String() string {
	bi := (big.Int)(tid)
	return bi.String()
}

// ToBigInt returns a *big.Int representation of the TableID.
func (tid TableID) ToBigInt() *big.Int {
	bi := (big.Int)(tid)
	b := &big.Int{}
	b.Set(&bi)
	return b
}

// NewTableID creates a TableID from a string representation of the uint256.
func NewTableID(strID string) (TableID, error) {
	tableID := &big.Int{}
	if _, ok := tableID.SetString(strID, 10); !ok {
		return TableID{}, fmt.Errorf("parsing stringified id failed")
	}
	if tableID.Cmp(&big.Int{}) < 0 {
		return TableID{}, fmt.Errorf("table id is negative")
	}
	return TableID(*tableID), nil
}
