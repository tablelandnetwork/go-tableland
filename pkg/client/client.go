package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

// Client is the Tableland client.
type Client struct {
	tblRPC      *rpc.Client
	tblContract *ethereum.Client
}

// Config configures the Client.
type Config struct {
	TblRPCClient      *rpc.Client
	TblContractClient *ethereum.Client
}

// NewClient creates a new Client.
func NewClient(config Config) *Client {
	return &Client{
		tblRPC:      config.TblRPCClient,
		tblContract: config.TblContractClient,
	}
}

// List lists something.
func (c *Client) List(ctx context.Context) error {
	return errors.New("not implemented")
}

// Create creates a new table on the Tableland.
func (c *Client) Create(ctx context.Context, createStatement string) (tables.Transaction, error) {
	req := &tableland.ValidateCreateTableRequest{CreateStatement: createStatement}
	var res tableland.ValidateCreateTableResponse

	if err := c.tblRPC.CallContext(ctx, &res, "tableland_validateCreateTable", req); err != nil {
		return nil, fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}

	t, err := c.tblContract.CreateTable(ctx, createStatement)
	if err != nil {
		return nil, fmt.Errorf("calling contract create table: %v", err)
	}

	return t, nil
}

// Read runs a read query and returns the results.
func (c *Client) Read(ctx context.Context, query string) (string, error) {
	req := &tableland.RunReadQueryRequest{Statement: query}
	var res tableland.RunReadQueryResponse

	if err := c.tblRPC.CallContext(ctx, &res, "runReadQuery", req); err != nil {
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

	if err := c.tblRPC.CallContext(ctx, &res, "relayWriteQuery", req); err != nil {
		return "", fmt.Errorf("calling rpc relayWriteQuery: %v", err)
	}

	return res.Transaction.Hash, nil
}

// Hash validates the provided create table statement and returns its hash.
func (c *Client) Hash(ctx context.Context, statement string) (string, error) {
	req := &tableland.ValidateCreateTableRequest{CreateStatement: statement}
	var res tableland.ValidateCreateTableResponse

	if err := c.tblRPC.CallContext(ctx, &res, "validateCreateTable", req); err != nil {
		return "", fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}

	return res.StructureHash, nil
}

// Receipt gets a transaction receipt.
func (c *Client) Receipt(ctx context.Context, txnHash string) (*tableland.TxnReceipt, bool, error) {
	req := tableland.GetReceiptRequest{TxnHash: txnHash}
	var res tableland.GetReceiptResponse

	if err := c.tblRPC.CallContext(ctx, &res, "getReceipt", req); err != nil {
		return nil, false, fmt.Errorf("calling rpc validateCreateTable: %v", err)
	}

	return res.Receipt, res.Ok, nil
}
