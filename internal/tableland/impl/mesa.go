package impl

import (
	"github.com/brunocalza/go-tableland/internal/tableland"
	"github.com/brunocalza/go-tableland/pkg/sqlstore"
	"github.com/brunocalza/go-tableland/pkg/tableregistry"
)

// TablelandMesa is the main implementation of Tableland spec
type TablelandMesa struct {
	store    sqlstore.SQLStore
	registry tableregistry.TableRegistry
}

func (t *TablelandMesa) CreateTable(args tableland.SQLArgs) (tableland.Response, error) {
	// execute sql statement
	return tableland.Response{Message: "Table created"}, nil
}

func (t *TablelandMesa) UpdateTable(args tableland.SQLArgs) (tableland.Response, error) {
	// check permission on Ethereum
	// execute sql statement
	return tableland.Response{Message: "Table updated"}, nil
}

func (t *TablelandMesa) RunSQL(args tableland.SQLArgs) (tableland.Response, error) {
	// check permission on Ethereum
	// execute sql statement
	// return data
	return tableland.Response{Message: "SQL executed"}, nil
}
