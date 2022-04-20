package tableland

import (
	"context"
	"fmt"
	"math/big"
)

// CreateTableRequest is a user CreateTable request.
type CreateTableRequest struct {
	Statement string `json:"statement"`
}

// CreateTableResponse is a CreateTable response.
type CreateTableResponse struct {
	StructureHash string `json:"structure_hash"`
}

// RunSQLRequest is a user RunSQL request.
type RunSQLRequest struct {
	Controller string `json:"controller"`
	Statement  string `json:"statement"`
}

// RunSQLResponse is a RunSQL response.
type RunSQLResponse struct {
	Result      interface{} `json:"data"`
	Transaction struct {
		Hash string `json:"hash"`
	} `json:"tx"`
}

// GetReceiptRequest is a GetTxnReceipt request.
type GetReceiptRequest struct {
	TxnHash string `json:"txn_hash"`
}

// TxnReceipt is a Tableland event processing receipt.
type TxnReceipt struct {
	ChainID     int64    `json:"chain_id"`
	TxnHash     string   `json:"txn_hash"`
	BlockNumber int64    `json:"block_number"`
	Error       *string  `json:"error,omitempty"`
	TableID     *TableID `json:"table_id,omitempty"`
}

// GetReceiptResponse is a GetTxnReceipt response.
type GetReceiptResponse struct {
	Ok      bool        `json:"ok"`
	Receipt *TxnReceipt `json:"receipt,omitempty"`
}

// CalculateTableHashRequest is a CreateTableHash request.
type CalculateTableHashRequest struct {
	CreateStatement string `json:"create_statement"`
}

// CalculateTableHashResponse is a CreateTableHash response.
type CalculateTableHashResponse struct {
	StructureHash string `json:"structure_hash"`
}

// AuthorizeRequest is a user Authorize request.
type AuthorizeRequest struct {
	Controller string `json:"controller"`
}

// SQLRunner defines the run SQL interface of Tableland.
type SQLRunner interface {
	RunSQL(context.Context, RunSQLRequest) (RunSQLResponse, error)
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	SQLRunner
	CreateTable(context.Context, CreateTableRequest) (CreateTableResponse, error)
	CalculateTableHash(context.Context, CalculateTableHashRequest) (CalculateTableHashResponse, error)
	Authorize(context.Context, AuthorizeRequest) error
	GetReceipt(context.Context, GetReceiptRequest) (GetReceiptResponse, error)
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
