package v1

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
	defaultTimeout := time.Minute
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
func (c *Client) Write(ctx context.Context, query string, opts ...WriteOption) (string, error) {
	config := defaultWriteConfig
	for _, opt := range opts {
		if err := opt(&config); err != nil {
			return "", fmt.Errorf("applying client write option: %s", err)
		}
	}

	tableID, err := c.Validate(query)
	if err != nil {
		return "", fmt.Errorf("calling Validate: %v", err)
	}
	res, err := c.tblContract.RunSQL(
		ctx,
		c.wallet.Address(),
		tables.TableID(tableID),
		query,
		tables.WithSuggestedPriceMultiplier(config.suggestedGasPriceMultiplier),
		tables.WithEstimatedGasLimitMultiplier(config.estimatedGasLimitMultiplier))
	if err != nil {
		return "", fmt.Errorf("calling RunSQL: %v", err)
	}
	return res.Hash().Hex(), nil
}

// WriteOption changes the behavior of the Write method.
type WriteOption func(*WriteConfig) error

// WriteConfig contains configuration attributes to call Write.
type WriteConfig struct {
	suggestedGasPriceMultiplier float64
	estimatedGasLimitMultiplier float64
}

var defaultWriteConfig = WriteConfig{
	suggestedGasPriceMultiplier: 1.0,
	estimatedGasLimitMultiplier: 1.0,
}

// WithSuggestedPriceMultiplier allows to modify the gas priced to be used with respect with the suggested gas price.
// For example, if `m=1.2` then the gas price to be used will be `suggestedGasPrice * 1.2`.
func WithSuggestedPriceMultiplier(m float64) WriteOption {
	return func(wc *WriteConfig) error {
		if m <= 0 {
			return fmt.Errorf("multiplier should be positive")
		}
		wc.suggestedGasPriceMultiplier = m

		return nil
	}
}

// WithEstimatedGasLimitMultiplier allows to modify the gas limit to be used with respect with the estimated gas.
// For example, if `m=1.2` then the gas limit to be used will be `estimatedGas * 1.2`.
func WithEstimatedGasLimitMultiplier(m float64) WriteOption {
	return func(wc *WriteConfig) error {
		if m <= 0 {
			return fmt.Errorf("multiplier should be positive")
		}
		wc.estimatedGasLimitMultiplier = m

		return nil
	}
}
