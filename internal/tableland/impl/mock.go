package impl

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
)

// TablelandMock is a dummy implementation of Tableland.
type TablelandMock struct{}

// CreateTable implements CreateTable.
func (t *TablelandMock) CreateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	return tableland.Response{Message: "Table created"}, nil
}

// UpdateTable implements UpdateTable.
func (t *TablelandMock) UpdateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	return tableland.Response{Message: "Table updated"}, nil
}

// RunSQL implements RunSQL.
func (t *TablelandMock) RunSQL(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	return tableland.Response{Message: "SQL executed"}, nil
}

// Authorize implements Authorize.
func (t *TablelandMock) Authorize(ctx context.Context, req tableland.Request) error {
	return nil
}
