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
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/siwe"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

// Client is the Tableland client.
type Client struct {
	tblRPC      *rpc.Client
	tblHTTP     *http.Client
	tblContract *ethereum.Client
	config      Config
}

// Config configures the Client.
type Config struct {
	TblAPIURL    string
	EthBackend   bind.ContractBackend
	ChainID      ChainID
	ContractAddr common.Address
	Wallet       *wallet.Wallet
}

// TxnReceipt is a Tableland event processing receipt.
type TxnReceipt struct {
	ChainID     ChainID `json:"chain_id"`
	TxnHash     string  `json:"txn_hash"`
	BlockNumber int64   `json:"block_number"`
	Error       *string `json:"error,omitempty"`
	TableID     *string `json:"table_id,omitempty"`
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

// ChainID is a supported EVM chain identifier.
type ChainID int64

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

// NewClient creates a new Client.
func NewClient(ctx context.Context, config Config) (*Client, error) {
	tblContract, err := ethereum.NewClient(
		config.EthBackend,
		tableland.ChainID(config.ChainID),
		config.ContractAddr,
		config.Wallet,
		impl.NewSimpleTracker(config.Wallet, config.EthBackend),
	)
	if err != nil {
		return nil, fmt.Errorf("creating contract client: %v", err)
	}

	siwe, err := siwe.EncodedSIWEMsg(tableland.ChainID(config.ChainID), config.Wallet, time.Hour*24*365)
	if err != nil {
		return nil, fmt.Errorf("creating siwe value: %v", err)
	}

	tblRPC, err := rpc.DialContext(ctx, config.TblAPIURL+"/rpc")
	if err != nil {
		return nil, fmt.Errorf("creating rpc client: %v", err)
	}
	tblRPC.SetHeader("Authorization", "Bearer "+siwe)

	return &Client{
		tblRPC:      tblRPC,
		tblHTTP:     &http.Client{},
		tblContract: tblContract,
		config:      config,
	}, nil
}

// List lists something.
func (c *Client) List(ctx context.Context) ([]TableInfo, error) {
	url := fmt.Sprintf(
		"%s/chain/%d/tables/controller/%s",
		c.config.TblAPIURL,
		c.config.ChainID,
		c.config.Wallet.Address().Hex(),
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

// Create creates a new table on the Tableland.
func (c *Client) Create(ctx context.Context, schema string, opts ...CreateOption) (TableID, string, error) {
	defaultTimeout := time.Minute * 10
	conf := createConfig{receiptTimeout: &defaultTimeout}
	for _, opt := range opts {
		opt(&conf)
	}

	createStatement := fmt.Sprintf("CREATE TABLE %s_%d %s", conf.prefix, c.config.ChainID, schema)
	req := &tableland.ValidateCreateTableRequest{CreateStatement: createStatement}
	var res tableland.ValidateCreateTableResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_validateCreateTable", req); err != nil {
		return TableID{}, "", fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}

	t, err := c.tblContract.CreateTable(ctx, c.config.Wallet.Address(), createStatement)
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

	return TableID(*tableID), fmt.Sprintf("%s_%d_%s", conf.prefix, c.config.ChainID, *r.TableID), nil
}

// Read runs a read query and returns the results.
func (c *Client) Read(ctx context.Context, query string) (string, error) {
	req := &tableland.RunReadQueryRequest{Statement: query}
	var res tableland.RunReadQueryResponse

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

// Write initiates a write query, returning the txn hash.
func (c *Client) Write(ctx context.Context, query string) (string, error) {
	req := &tableland.RelayWriteQueryRequest{Statement: query}
	var res tableland.RelayWriteQueryResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_relayWriteQuery", req); err != nil {
		return "", fmt.Errorf("calling rpc relayWriteQuery: %v", err)
	}

	return res.Transaction.Hash, nil
}

// Hash validates the provided create table statement and returns its hash.
func (c *Client) Hash(ctx context.Context, statement string) (string, error) {
	req := &tableland.ValidateCreateTableRequest{CreateStatement: statement}
	var res tableland.ValidateCreateTableResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_validateCreateTable", req); err != nil {
		return "", fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}

	return res.StructureHash, nil
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
	req := tableland.SetControllerRequest{Controller: controller.Hex(), TokenID: tableID.String()}
	var res tableland.SetControllerResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_setController", req); err != nil {
		return "", fmt.Errorf("calling rpc setController: %v", err)
	}

	return res.Transaction.Hash, nil
}

func (c *Client) getReceipt(ctx context.Context, txnHash string) (*TxnReceipt, bool, error) {
	req := tableland.GetReceiptRequest{TxnHash: txnHash}
	var res tableland.GetReceiptResponse
	if err := c.tblRPC.CallContext(ctx, &res, "tableland_getReceipt", req); err != nil {
		return nil, false, fmt.Errorf("calling rpc getReceipt: %v", err)
	}
	if !res.Ok {
		return nil, res.Ok, nil
	}
	receipt := TxnReceipt{
		ChainID:     ChainID(res.Receipt.ChainID),
		TxnHash:     res.Receipt.TxnHash,
		BlockNumber: res.Receipt.BlockNumber,
		Error:       res.Receipt.Error,
		TableID:     res.Receipt.TableID,
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
