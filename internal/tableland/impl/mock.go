package impl

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
)

// TablelandMock is a dummy implementation of Tableland.
type TablelandMock struct{}

// CreateTable implements CreateTable.
func (t *TablelandMock) CreateTable(
	ctx context.Context,
	req tableland.CreateTableRequest) (tableland.CreateTableResponse, error) {
	return tableland.CreateTableResponse{}, nil
}

// CalculateTableHash implements CalculateTableHash.
func (t *TablelandMock) CalculateTableHash(
	ctx context.Context,
	req tableland.CalculateTableHashRequest) (tableland.CalculateTableHashResponse, error) {
	return tableland.CalculateTableHashResponse{}, nil
}

// RunSQL implements RunSQL.
func (t *TablelandMock) RunSQL(
	ctx context.Context,
	req tableland.RunSQLRequest) (tableland.RunSQLResponse, error) {
	return tableland.RunSQLResponse{}, nil
}

// Authorize implements Authorize.
func (t *TablelandMock) Authorize(ctx context.Context, req tableland.AuthorizeRequest) error {
	return nil
}

// GetReceipt implements GetRrceipt.
func (t *TablelandMock) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest) (tableland.GetReceiptResponse, error) {
	return tableland.GetReceiptResponse{Ok: false}, nil
}
