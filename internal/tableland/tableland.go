package tableland

import (
	"context"
	"fmt"
	"math/big"
)

// RunSQLRequest is a user RunSQL request.
type RunSQLRequest struct {
	Statement string `json:"statement"`
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
	ChainID     ChainID `json:"chain_id"`
	TxnHash     string  `json:"txn_hash"`
	BlockNumber int64   `json:"block_number"`
	Error       *string `json:"error,omitempty"`
	TableID     *string `json:"table_id,omitempty"`
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

// SetControllerRequest is a user SetController request.
type SetControllerRequest struct {
	Caller     string `json:"caller"`
	Controller string `json:"controller"`
	TokenID    string `json:"tokenId"`
}

// SetControllerResponse is a RunSQL response.
type SetControllerResponse struct {
	Transaction struct {
		Hash string `json:"hash"`
	} `json:"tx"`
}

// SQLRunner defines the run SQL interface of Tableland.
type SQLRunner interface {
	RunSQL(context.Context, RunSQLRequest) (RunSQLResponse, error)
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	SQLRunner
	ValidateCreateTable(context.Context, ValidateCreateTableRequest) (ValidateCreateTableResponse, error)
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

// ChainID is a supported EVM chain identifier.
type ChainID int64
