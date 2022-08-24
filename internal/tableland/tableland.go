package tableland

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

// RelayWriteQueryRequest is a user RelayWriteQuery request.
type RelayWriteQueryRequest struct {
	Statement string `json:"statement"`
}

// RelayWriteQueryResponse is a RelayWriteQuery response.
type RelayWriteQueryResponse struct {
	Transaction struct {
		Hash string `json:"hash"`
	} `json:"tx"`
}

// RunReadQueryRequest is a user RunReadQuery request.
type RunReadQueryRequest struct {
	Statement string `json:"statement"`
}

// RunReadQueryResponse is a RunReadQuery response.
type RunReadQueryResponse struct {
	Result interface{} `json:"data"`
}

// GetReceiptRequest is a GetTxnReceipt request.
type GetReceiptRequest struct {
	TxnHash string `json:"txn_hash"`
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

// GetReceiptResponse is a GetTxnReceipt response.
type GetReceiptResponse struct {
	Ok      bool        `json:"ok"`
	Receipt *TxnReceipt `json:"receipt,omitempty"`
}

// ValidateCreateTableRequest is a ValidateCreateTable request.
type ValidateCreateTableRequest struct {
	CreateStatement string `json:"create_statement"`
}

// ValidateCreateTableResponse is a ValidateCreateTable response.
type ValidateCreateTableResponse struct {
	StructureHash string `json:"structure_hash"`
}

// ValidateWriteQueryRequest is a ValidateWriteQuery request.
type ValidateWriteQueryRequest struct {
	Statement string `json:"statement"`
}

// ValidateWriteQueryResponse is a ValidateWriteQuery response.
type ValidateWriteQueryResponse struct {
	TableID string `json:"table_id"`
}

// SetControllerRequest is a user SetController request.
type SetControllerRequest struct {
	Controller string `json:"controller"`
	TokenID    string `json:"token_id"`
}

// SetControllerResponse is a RunSQL response.
type SetControllerResponse struct {
	Transaction struct {
		Hash string `json:"hash"`
	} `json:"tx"`
}

// SQLRunner defines the run SQL interface of Tableland.
type SQLRunner interface {
	RunReadQuery(context.Context, RunReadQueryRequest) (RunReadQueryResponse, error)
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	SQLRunner
	ValidateCreateTable(context.Context, ValidateCreateTableRequest) (ValidateCreateTableResponse, error)
	ValidateWriteQuery(context.Context, ValidateWriteQueryRequest) (ValidateWriteQueryResponse, error)
	RelayWriteQuery(context.Context, RelayWriteQueryRequest) (RelayWriteQueryResponse, error)
	GetReceipt(context.Context, GetReceiptRequest) (GetReceiptResponse, error)
	SetController(context.Context, SetControllerRequest) (SetControllerResponse, error)
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

// NewTableIDFromInt64 returns a TableID from a int64.
func NewTableIDFromInt64(intID int64) (TableID, error) {
	tableID := &big.Int{}
	tableID.SetInt64(intID)
	return TableID(*tableID), nil
}

// ChainID is a supported EVM chain identifier.
type ChainID int64

// Table represents a database table.
type Table struct {
	id      TableID
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

	tableID, err := NewTableID(parts[len(parts)-1])
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
