package tables

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
)

// TableID is the ID of a Table.
type TableID big.Int

// TableIDs is a list of TableID.
type TableIDs []TableID

// String transform a list of TableIds into a string.
func (ids TableIDs) String() string {
	tableIdsStr := make([]string, len(ids))
	for i, tableID := range ids {
		tableIdsStr[i] = tableID.String()
	}
	return strings.Join(tableIdsStr, ",")
}

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

// NewTableIDFromInt64 returns a TableID from a int64.
func NewTableIDFromInt64(intID int64) (TableID, error) {
	tableID := &big.Int{}
	tableID.SetInt64(intID)
	return TableID(*tableID), nil
}

// Transaction represents a Smart Contract transaction.
type Transaction interface {
	Hash() common.Hash
}

// TablelandTables defines the interface for interaction with the TablelandTables smart contract.
type TablelandTables interface {
	// CreateTable mints a new table NFT.
	CreateTable(context.Context, common.Address, string) (Transaction, error)

	// IsOwner checks if the provided address is the owner of the provided table.
	IsOwner(context.Context, common.Address, *big.Int) (bool, error)

	// RunSQL sends a transaction with a SQL statement to the Tabeland Smart Contract.
	RunSQL(context.Context, common.Address, TableID, string, ...RunSQLOption) (Transaction, error)

	// SetController sends a transaction that sets the controller for a token id in Smart Contract.
	SetController(context.Context, common.Address, TableID, common.Address) (Transaction, error)
}

// RunSQLOption changes the behavior of the Write method.
type RunSQLOption func(*RunSQLConfig) error

// RunSQLConfig contains configuration attributes to call Write.
type RunSQLConfig struct {
	SuggestedGasPriceMultiplier float64
	EstimatedGasLimitMultiplier float64
}

// DefaultRunSQLConfig is the default configuration for RunSQL if no options are passed.
var DefaultRunSQLConfig = RunSQLConfig{
	SuggestedGasPriceMultiplier: 1.0,
	EstimatedGasLimitMultiplier: 1.0,
}

// WithSuggestedPriceMultiplier allows to modify the gas priced to be used with respect with the suggested gas price.
// For example, if `m=1.2` then the gas price to be used will be `suggestedGasPrice * 1.2`.
func WithSuggestedPriceMultiplier(m float64) RunSQLOption {
	return func(wc *RunSQLConfig) error {
		if m <= 0 {
			return fmt.Errorf("multiplier should be positive")
		}
		wc.SuggestedGasPriceMultiplier = m

		return nil
	}
}

// WithEstimatedGasLimitMultiplier allows to modify the gas limit to be used with respect with the estimated gas.
// For example, if `m=1.2` then the gas limit to be used will be `estimatedGas * 1.2`.
func WithEstimatedGasLimitMultiplier(m float64) RunSQLOption {
	return func(wc *RunSQLConfig) error {
		if m <= 0 {
			return fmt.Errorf("multiplier should be positive")
		}
		wc.EstimatedGasLimitMultiplier = m

		return nil
	}
}
