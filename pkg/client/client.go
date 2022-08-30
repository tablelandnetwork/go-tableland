package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/internal/router/rpcservice"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/siwe"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

var defaultChain = Chains.PolygonMumbai

// TxnReceipt is a Tableland event processing receipt.
type TxnReceipt struct {
	ChainID       ChainID `json:"chain_id"`
	TxnHash       string  `json:"txn_hash"`
	BlockNumber   int64   `json:"block_number"`
	Error         string  `json:"error"`
	ErrorEventIdx int     `json:"error_event_idx"`
	TableID       *string `json:"table_id,omitempty"`
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

// TableInfo summarizes information about a table.
type TableInfo struct {
	Controller string    `json:"controller"`
	Name       string    `json:"name"`
	Structure  string    `json:"structure"`
	CreatedAt  time.Time `json:"created_at"`
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

// Client is the Tableland client.
type Client struct {
	tblRPC      *rpc.Client
	tblHTTP     *http.Client
	tblContract *ethereum.Client
	chain       Chain
	relayWrites bool
	wallet      *wallet.Wallet
}

type config struct {
	chain           *Chain
	relayWrites     *bool
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

// NewClientRelayWrites specifies whether or not to relay write queries through the Tableland validator.
func NewClientRelayWrites(relay bool) NewClientOption {
	return func(ncc *config) {
		ncc.relayWrites = &relay
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
	var relay bool
	if config.relayWrites != nil {
		relay = *config.relayWrites
	} else {
		relay = config.chain.CanRelayWrites()
	}
	if relay && !config.chain.CanRelayWrites() {
		return nil, errors.New("options specified to relay writes for a chain that doesn't support it")
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

	siwe, err := siwe.EncodedSIWEMsg(tableland.ChainID(config.chain.ID), wallet, time.Hour*24*365)
	if err != nil {
		return nil, fmt.Errorf("creating siwe value: %v", err)
	}

	tblRPC, err := rpc.DialContext(ctx, config.chain.Endpoint+"/rpc")
	if err != nil {
		return nil, fmt.Errorf("creating rpc client: %v", err)
	}
	tblRPC.SetHeader("Authorization", "Bearer "+siwe)

	return &Client{
		tblRPC:      tblRPC,
		tblHTTP:     &http.Client{},
		tblContract: tblContract,
		chain:       *config.chain,
		relayWrites: relay,
		wallet:      wallet,
	}, nil
}

// List lists something.
func (c *Client) List(ctx context.Context) ([]TableInfo, error) {
	url := fmt.Sprintf(
		"%s/chain/%d/tables/controller/%s",
		c.chain.Endpoint,
		c.chain.ID,
		c.wallet.Address().Hex(),
	)
	res, err := c.tblHTTP.Get(url)
	if err != nil {
		return nil, fmt.Errorf("calling http endpoint: %v", err)
	}
	defer func() {
		_ = res.Body.Close()
	}()

	var ret []TableInfo

	if err := json.NewDecoder(res.Body).Decode(&ret); err != nil {
		return nil, fmt.Errorf("decoding response body: %v", err)
	}

	return ret, nil
}

type createConfig struct {
	prefix         string
	receiptTimeout *time.Duration
}

// CreateOption controls the behavior of Create.
type CreateOption func(*createConfig)

// WithPrefix allows you to specify an optional table name prefix where
// the final table name will be <prefix>_<chain-id>_<tableid>.
func WithPrefix(prefix string) CreateOption {
	return func(cc *createConfig) {
		cc.prefix = prefix
	}
}

// WithReceiptTimeout specifies how long to wait for the Tableland
// receipt that contains the table id.
func WithReceiptTimeout(timeout time.Duration) CreateOption {
	return func(cc *createConfig) {
		cc.receiptTimeout = &timeout
	}
}

// Create creates a new table on the rpcservice.
func (c *Client) Create(ctx context.Context, schema string, opts ...CreateOption) (TableID, string, error) {
	defaultTimeout := time.Minute * 10
	conf := createConfig{receiptTimeout: &defaultTimeout}
	for _, opt := range opts {
		opt(&conf)
	}

	createStatement := fmt.Sprintf("CREATE TABLE %s_%d %s", conf.prefix, c.chain.ID, schema)
	req := &rpcservice.ValidateCreateTableRequest{CreateStatement: createStatement}
	var res rpcservice.ValidateCreateTableResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_validateCreateTable", req); err != nil {
		return TableID{}, "", fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}

	t, err := c.tblContract.CreateTable(ctx, c.wallet.Address(), createStatement)
	if err != nil {
		return TableID{}, "", fmt.Errorf("calling contract create table: %v", err)
	}

	r, found, err := c.waitForReceipt(ctx, t.Hash().Hex(), *conf.receiptTimeout)
	if err != nil {
		return TableID{}, "", fmt.Errorf("waiting for txn receipt: %v", err)
	}
	if !found {
		return TableID{}, "", errors.New("no receipt found before timeout")
	}

	tableID, ok := big.NewInt(0).SetString(*r.TableID, 10)
	if !ok {
		return TableID{}, "", errors.New("parsing table id from response")
	}

	return TableID(*tableID), fmt.Sprintf("%s_%d_%s", conf.prefix, c.chain.ID, *r.TableID), nil
}

// Read runs a read query and returns the results.
func (c *Client) Read(ctx context.Context, query string) (string, error) {
	req := &rpcservice.RunReadQueryRequest{Statement: query}
	var res rpcservice.RunReadQueryResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_runReadQuery", req); err != nil {
		return "", fmt.Errorf("calling rpc runReadQuery: %v", err)
	}

	b, err := json.Marshal(res.Result)
	if err != nil {
		return "", fmt.Errorf("marshaling read result: %v", err)
	}

	// TODO: Make this do something better than returning a json string.
	return string(b), nil
}

type writeConfig struct {
	relay bool
}

// WriteOption controls the behavior of Write.
type WriteOption func(*writeConfig)

// WriteRelay specifies whether or not to relay write queries through the Tableland validator.
// Default behavior is false for main net EVM chains, true for all others.
func WriteRelay(relay bool) WriteOption {
	return func(wc *writeConfig) {
		wc.relay = relay
	}
}

// Write initiates a write query, returning the txn hash.
func (c *Client) Write(ctx context.Context, query string, opts ...WriteOption) (string, error) {
	conf := writeConfig{relay: c.relayWrites}
	for _, opt := range opts {
		opt(&conf)
	}
	if conf.relay {
		req := &rpcservice.RelayWriteQueryRequest{Statement: query}
		var res rpcservice.RelayWriteQueryResponse
		if err := c.tblRPC.CallContext(ctx, &res, "tableland_relayWriteQuery", req); err != nil {
			return "", fmt.Errorf("calling rpc relayWriteQuery: %v", err)
		}
		return res.Transaction.Hash, nil
	}
	tableID, err := c.Validate(ctx, query)
	if err != nil {
		return "", fmt.Errorf("calling Validate: %v", err)
	}
	res, err := c.tblContract.RunSQL(ctx, c.wallet.Address(), tables.TableID(tableID), query)
	if err != nil {
		return "", fmt.Errorf("calling RunSQL: %v", err)
	}
	return res.Hash().Hex(), nil
}

// Hash validates the provided create table statement and returns its hash.
func (c *Client) Hash(ctx context.Context, statement string) (string, error) {
	req := &rpcservice.ValidateCreateTableRequest{CreateStatement: statement}
	var res rpcservice.ValidateCreateTableResponse
	if err := c.tblRPC.CallContext(ctx, &res, "tableland_validateCreateTable", req); err != nil {
		return "", fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}
	return res.StructureHash, nil
}

// Validate validates a write query, returning the table id.
func (c *Client) Validate(ctx context.Context, statement string) (TableID, error) {
	req := &rpcservice.ValidateWriteQueryRequest{Statement: statement}
	var res rpcservice.ValidateWriteQueryResponse
	if err := c.tblRPC.CallContext(ctx, &res, "tableland_validateWriteQuery", req); err != nil {
		return TableID{}, fmt.Errorf("calling rpc validateWriteQuery: %v", err)
	}
	tableID, ok := big.NewInt(0).SetString(res.TableID, 10)
	if !ok {
		return TableID{}, errors.New("parsing table id from response")
	}

	return TableID(*tableID), nil
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
	req := rpcservice.SetControllerRequest{Controller: controller.Hex(), TokenID: tableID.String()}
	var res rpcservice.SetControllerResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_setController", req); err != nil {
		return "", fmt.Errorf("calling rpc setController: %v", err)
	}

	return res.Transaction.Hash, nil
}

func (c *Client) getReceipt(ctx context.Context, txnHash string) (*TxnReceipt, bool, error) {
	req := rpcservice.GetReceiptRequest{TxnHash: txnHash}
	var res rpcservice.GetReceiptResponse
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
		tmpl, found := infuraURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Infura", config.chain.ID)
		}
		return ethclient.DialContext(ctx, fmt.Sprintf(tmpl, config.infuraAPIKey))
	} else if config.alchemyAPIKey != "" && config.contractBackend == nil && config.infuraAPIKey == "" {
		tmpl, found := alchemyURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Alchemy", config.chain.ID)
		}
		return ethclient.DialContext(ctx, fmt.Sprintf(tmpl, config.alchemyAPIKey))
	} else if config.local {
		url, found := localURLs[config.chain.ID]
		if !found {
			return nil, fmt.Errorf("chain id %v not supported for Local", config.chain.ID)
		}
		return ethclient.DialContext(ctx, url)
	}
	return nil, errors.New("no provider specified, must provide an Infura API key, Alchemy API key, or an ETH backend")
}
