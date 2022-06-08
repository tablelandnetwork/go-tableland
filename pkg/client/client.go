package client

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/spruceid/siwe-go"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/rpc"
	"github.com/textileio/go-tableland/internal/tableland"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/wallet"
)

// Client is the Tableland client.
type Client struct {
	tblRPC      *rpc.Client
	tblContract *ethereum.Client
}

// Config configures the Client.
type Config struct {
	TablelandAPI      string
	EthereumAPI       string
	TablelandContract common.Address
	ChainID           int64
	Wallet            *wallet.Wallet
}

// NewClient creates a new Client.
func NewClient(
	ctx context.Context,
	config Config,
) (*Client, error) {
	tblRPC, err := rpc.DialContext(ctx, config.TablelandAPI)
	if err != nil {
		return nil, fmt.Errorf("creating rpc client: %v", err)
	}
	bearer, err := bearerValue(config.ChainID, config.Wallet)
	if err != nil {
		return nil, fmt.Errorf("getting bearer value: %v", err)
	}
	tblRPC.SetHeader("Authorization", bearer)

	ethClient, err := ethclient.DialContext(ctx, config.EthereumAPI)
	if err != nil {
		return nil, fmt.Errorf("creating ethereum client: %v", err)
	}

	tracker := nonceimpl.NewSimpleTracker(config.Wallet, ethClient)

	tblContract, err := ethereum.NewClient(
		ethClient,
		tableland.ChainID(config.ChainID),
		config.TablelandContract,
		config.Wallet,
		tracker,
	)
	if err != nil {
		return nil, fmt.Errorf("creating tableland contract client: %v", err)
	}

	return &Client{
		tblRPC:      tblRPC,
		tblContract: tblContract,
	}, nil
}

// List lists something.
func (c *Client) List(ctx context.Context) error {
	return errors.New("not implemented")
}

// Create creates a new table on the Tableland.
func (c *Client) Create(ctx context.Context, createStatement string) (tables.Transaction, error) {
	req := &tableland.ValidateCreateTableRequest{CreateStatement: createStatement}
	var res tableland.ValidateCreateTableResponse

	if err := c.tblRPC.CallContext(ctx, &res, "validateCreateTable", req); err != nil {
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

func bearerValue(chainID int64, wallet *wallet.Wallet) (string, error) {
	validFor := time.Hour * 24 * 365
	opts := map[string]interface{}{
		"chainId":        chainID,
		"expirationTime": time.Now().Add(validFor),
		"nonce":          siwe.GenerateNonce(),
	}

	msg, err := siwe.InitMessage("Tableland", wallet.Address().Hex(), "https://tableland.xyz", "1", opts)
	if err != nil {
		return "", fmt.Errorf("initializing siwe message: %v", err)
	}

	payload := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(msg.String()), msg.String())
	hash := crypto.Keccak256Hash([]byte(payload))
	signature, err := crypto.Sign(hash.Bytes(), wallet.PrivateKey())
	if err != nil {
		return "", fmt.Errorf("signing siwe message: %v", err)
	}
	signature[64] += 27

	bearerValue := struct {
		Message   string `json:"message"`
		Signature string `json:"signature"`
	}{Message: msg.String(), Signature: hexutil.Encode(signature)}
	bearer, err := json.Marshal(bearerValue)
	if err != nil {
		return "", fmt.Errorf("json marshaling signed siwe: %v", err)
	}

	return fmt.Sprintf("Bearer %s", base64.StdEncoding.EncodeToString(bearer)), nil
}
