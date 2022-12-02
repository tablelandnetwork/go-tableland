package client

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/tables"
)

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

	createStatement := fmt.Sprintf("CREATE TABLE %s_%d %s", conf.prefix, c.chain.ID, schema)
	if _, err := c.parser.ValidateCreateTable(createStatement, tableland.ChainID(c.chain.ID)); err != nil {
		return TableID{}, "", fmt.Errorf("invalid create statement: %s", err)
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

	tableID, ok := big.NewInt(0).SetString(r.TableId, 10)
	if !ok {
		return TableID{}, "", errors.New("parsing table id from response")
	}

	return TableID(*tableID), fmt.Sprintf("%s_%d_%s", conf.prefix, c.chain.ID, r.TableId), nil
}

// Write initiates a write query, returning the txn hash.
func (c *Client) Write(ctx context.Context, query string) (string, error) {
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
