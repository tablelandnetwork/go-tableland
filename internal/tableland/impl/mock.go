package impl

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
)

// TablelandMock is a dummy implementation of Tableland
type TablelandMock struct{}

func (t *TablelandMock) CreateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	return tableland.Response{Message: "Table created"}, nil
}

func (t *TablelandMock) UpdateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	return tableland.Response{Message: "Table updated"}, nil
}

func (t *TablelandMock) RunSQL(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	return tableland.Response{Message: "SQL executed"}, nil
}
