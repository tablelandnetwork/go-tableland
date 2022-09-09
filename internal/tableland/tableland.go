package tableland

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/pkg/tables"
)

// Column defines a column in table data.
type Column struct {
	Name string `json:"name"`
}

// TableData defines a tabular representation of query results.
type TableData struct {
	Columns []Column         `json:"columns"`
	Rows    [][]*ColumnValue `json:"rows"`
}

// ColumnValue wraps data from the db that may be raw json or any other value.
type ColumnValue struct {
	jsonValue  json.RawMessage
	otherValue interface{}
}

// Value returns the underlying value.
func (cv *ColumnValue) Value() interface{} {
	if cv.jsonValue != nil {
		return cv.jsonValue
	}
	return cv.otherValue
}

// Scan implements Scan.
func (cv *ColumnValue) Scan(src interface{}) error {
	cv.jsonValue = nil
	cv.otherValue = nil
	switch src := src.(type) {
	case string:
		trimmed := strings.TrimLeft(src, " ")
		if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(src)) {
			cv.jsonValue = []byte(src)
		} else {
			cv.otherValue = src
		}
	case []byte:
		tmp := make([]byte, len(src))
		copy(tmp, src)
		cv.otherValue = tmp
	default:
		cv.otherValue = src
	}
	return nil
}

// MarshalJSON implements MarshalJSON.
func (cv *ColumnValue) MarshalJSON() ([]byte, error) {
	if cv.jsonValue != nil {
		return cv.jsonValue, nil
	}
	return json.Marshal(cv.otherValue)
}

// JSONColValue creates a UserValue with the provided json.
func JSONColValue(v json.RawMessage) *ColumnValue {
	return &ColumnValue{jsonValue: v}
}

// OtherColValue creates a UserValue with the provided other value.
func OtherColValue(v interface{}) *ColumnValue {
	return &ColumnValue{otherValue: v}
}

// TxnReceipt is a Tableland event processing receipt.
type TxnReceipt struct {
	ChainID     ChainID `json:"chain_id"`
	TxnHash     string  `json:"txn_hash"`
	BlockNumber int64   `json:"block_number"`

	TableID       *string `json:"table_id,omitempty"`
	Error         string  `json:"error"`
	ErrorEventIdx int     `json:"error_event_idx"`
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	RunReadQuery(ctx context.Context, stmt string) (*TableData, error)
	ValidateCreateTable(ctx context.Context, chainID ChainID, stmt string) (string, error)
	ValidateWriteQuery(ctx context.Context, chainID ChainID, stmt string) (tables.TableID, error)
	RelayWriteQuery(
		ctx context.Context,
		chainID ChainID,
		caller common.Address,
		stmt string,
	) (tables.Transaction, error)
	GetReceipt(ctx context.Context, chainID ChainID, txnHash string) (bool, *TxnReceipt, error)
	SetController(
		ctx context.Context,
		chainID ChainID,
		caller common.Address,
		controller common.Address,
		tableID tables.TableID,
	) (tables.Transaction, error)
}

// ChainID is a supported EVM chain identifier.
type ChainID int64

// Table represents a database table.
type Table struct {
	id      tables.TableID
	prefix  string
	chainID ChainID
}

// ChainID returns table's chain id.
func (t Table) ChainID() ChainID {
	return t.chainID
}

// NewTableFromName creates a Table from its name.
func NewTableFromName(name string) (Table, error) {
	parts := strings.Split(name, "_")

	if len(parts) < 2 {
		return Table{}, errors.New("table name has invalid format")
	}

	tableID, err := tables.NewTableID(parts[len(parts)-1])
	if err != nil {
		return Table{}, fmt.Errorf("new table id: %s", err)
	}

	i, err := strconv.ParseInt(parts[len(parts)-2], 10, 64)
	if err != nil {
		return Table{}, fmt.Errorf("parse chain id: %s", err)
	}

	return Table{
		id:      tableID,
		prefix:  strings.Join(parts[:len(parts)-2], "_"),
		chainID: ChainID(i),
	}, nil
}

// EVMEvent is a Tableland on-chain event produced by the Registry SC.
type EVMEvent struct {
	Address     common.Address
	Topics      []byte
	Data        []byte
	BlockNumber uint64
	TxHash      common.Hash
	TxIndex     uint
	BlockHash   common.Hash
	Index       uint

	// Enhanced fields
	ChainID   ChainID
	EventJSON []byte
	EventType string
}

// EVMBlockInfo contains information about an EVM block.
type EVMBlockInfo struct {
	ChainID     ChainID
	BlockNumber int64
	Timestamp   time.Time
}
