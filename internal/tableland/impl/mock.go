package impl

import (
	"github.com/textileio/go-tableland/internal/tableland"
)

// TablelandMock is a dummy implementation of Tableland
type TablelandMock struct{}

func (t *TablelandMock) CreateTable(args tableland.SQLArgs) (tableland.Response, error) {
	return tableland.Response{Message: "Table created"}, nil
}

func (t *TablelandMock) UpdateTable(args tableland.SQLArgs) (tableland.Response, error) {
	return tableland.Response{Message: "Table updated"}, nil
}

func (t *TablelandMock) RunSQL(args tableland.SQLArgs) (tableland.Response, error) {
	return tableland.Response{Message: "SQL executed"}, nil
}
