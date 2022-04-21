package impl

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
)

// TablelandMock is a dummy implementation of Tableland.
type TablelandMock struct{}

// ValidateCreateTable implements ValidateCreateTable.
func (t *TablelandMock) ValidateCreateTable(
	ctx context.Context,
	req tableland.ValidateCreateTableRequest) (tableland.ValidateCreateTableResponse, error) {
	return tableland.ValidateCreateTableResponse{}, nil
}

// RunSQL implements RunSQL.
func (t *TablelandMock) RunSQL(
	ctx context.Context,
	req tableland.RunSQLRequest) (tableland.RunSQLResponse, error) {
	return tableland.RunSQLResponse{}, nil
}

// GetReceipt implements GetRrceipt.
func (t *TablelandMock) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest) (tableland.GetReceiptResponse, error) {
	return tableland.GetReceiptResponse{Ok: false}, nil
}
