package impl

import (
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

// TablelandMesa is the main implementation of Tableland spec
type TablelandMesa struct {
	Store    sqlstore.SQLStore
	Registry tableregistry.TableRegistry
}

func (t *TablelandMesa) CreateTable(args tableland.SQLArgs) (tableland.Response, error) {
	err := t.Store.Query(args.Statement)
	if err == nil {
		return tableland.Response{Message: "Table created"}, nil
	}
	return tableland.Response{Message: err.Error()}, err
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
