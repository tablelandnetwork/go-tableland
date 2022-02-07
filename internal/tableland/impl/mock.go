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
