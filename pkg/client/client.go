package client

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/textileio/go-tableland/internal/router/controllers/legacy"
	systemimpl "github.com/textileio/go-tableland/internal/system/impl"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var defaultChain = Chains.PolygonMumbai

// Client is the Tableland client.
type Client struct {
	tblHTTP     *http.Client
	tblContract *ethereum.Client
	chain       Chain
	wallet      *wallet.Wallet
	parser      parsing.SQLValidator
	baseURL     *url.URL
}

type config struct {
	chain           *Chain
	infuraAPIKey    string
	alchemyAPIKey   string
	local           bool
	contractBackend bind.ContractBackend
}

// NewClientOption controls the behavior of NewClient.
type NewClientOption func(*config)

// NewClientChain specifies chaininfo.
func NewClientChain(chain Chain) NewClientOption {
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
		systemimpl.SystemTablesPrefix,
		systemimpl.RegistryTableName,
	}, parserOpts...)
	if err != nil {
		return nil, fmt.Errorf("new parser: %s", err)
	}

	parser, err = parserimpl.NewInstrumentedSQLValidator(parser)
	if err != nil {
		return nil, fmt.Errorf("instrumenting parser: %s", err)
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

type receiptConfig struct {
	timeout *time.Duration
}

// ReceiptOption controls the behavior of calls to Receipt.
type ReceiptOption func(*receiptConfig)

// WaitFor causes calls to Receipt to wait for the specified duration.
func WaitFor(timeout time.Duration) ReceiptOption {
	return func(rc *receiptConfig) {
		rc.timeout = &timeout
	}
}

// Receipt gets a transaction receipt.
func (c *Client) Receipt(
	ctx context.Context,
	txnHash string,
	options ...ReceiptOption,
) (*TxnReceipt, bool, error) {
	config := receiptConfig{}
	for _, option := range options {
		option(&config)
	}
	if config.timeout != nil {
		return c.waitForReceipt(ctx, txnHash, *config.timeout)
	}
	return c.getReceipt(ctx, txnHash)
}

// SetController sets the controller address for the specified table.
func (c *Client) SetController(
	ctx context.Context,
	controller common.Address,
	tableID TableID,
) (string, error) {
	req := legacy.SetControllerRequest{Controller: controller.Hex(), TokenID: tableID.String()}
	var res legacy.SetControllerResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_setController", req); err != nil {
		return "", fmt.Errorf("calling rpc setController: %v", err)
	}

	return res.Transaction.Hash, nil
}

func (c *Client) getReceipt(ctx context.Context, txnHash string) (*TxnReceipt, bool, error) {
	req := legacy.GetReceiptRequest{TxnHash: txnHash}
	var res legacy.GetReceiptResponse
	if err := c.tblRPC.CallContext(ctx, &res, "tableland_getReceipt", req); err != nil {
		return nil, false, fmt.Errorf("calling rpc getReceipt: %v", err)
	}
	if !res.Ok {
		return nil, res.Ok, nil
	}

	receipt := TxnReceipt{
		ChainID:       ChainID(res.Receipt.ChainID),
		TxnHash:       res.Receipt.TxnHash,
		BlockNumber:   res.Receipt.BlockNumber,
		Error:         res.Receipt.Error,
		ErrorEventIdx: res.Receipt.ErrorEventIdx,
		TableID:       res.Receipt.TableID,
	}
	return &receipt, res.Ok, nil
}

func (c *Client) waitForReceipt(
	ctx context.Context,
	txnHash string,
	timeout time.Duration,
) (*TxnReceipt, bool, error) {
	for stay, timeout := true, time.After(timeout); stay; {
		select {
		case <-timeout:
			stay = false
		default:
			receipt, found, err := c.getReceipt(ctx, txnHash)
			if err != nil {
				return nil, false, err
			}
			if found {
				return receipt, found, nil
			}
			time.Sleep(time.Second)
		}
	}
	return nil, false, nil
}

// Close implements Close.
func (c *Client) Close() {
	c.tblRPC.Close()
}

func getContractBackend(ctx context.Context, config config) (bind.ContractBackend, error) {
	if config.contractBackend != nil && config.infuraAPIKey == "" && config.alchemyAPIKey == "" {
		return config.contractBackend, nil
	} else if config.infuraAPIKey != "" && config.contractBackend == nil && config.alchemyAPIKey == "" {
		tmpl, found := InfuraURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Infura", config.chain.ID)
		}
		return ethclient.DialContext(ctx, fmt.Sprintf(tmpl, config.infuraAPIKey))
	} else if config.alchemyAPIKey != "" && config.contractBackend == nil && config.infuraAPIKey == "" {
		tmpl, found := AlchemyURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Alchemy", config.chain.ID)
		}
		return ethclient.DialContext(ctx, fmt.Sprintf(tmpl, config.alchemyAPIKey))
	} else if config.local {
		url, found := LocalURLs[config.chain.ID]
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
