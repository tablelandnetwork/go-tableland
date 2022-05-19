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

// RunReadQuery implements RunReadQuery.
func (t *TablelandMock) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest) (tableland.RunReadQueryResponse, error) {
	return tableland.RunReadQueryResponse{}, nil
}

// RelayWriteQuery implements RelayWriteQuery.
func (t *TablelandMock) RunSQL(
	ctx context.Context,
	req tableland.RelayWriteQueryRequest) (tableland.RelayWriteQueryResponse, error) {
	return tableland.RelayWriteQueryResponse{}, nil
}

// GetReceipt implements GetRrceipt.
func (t *TablelandMock) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest) (tableland.GetReceiptResponse, error) {
	return tableland.GetReceiptResponse{Ok: false}, nil
}
