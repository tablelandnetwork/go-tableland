package impl

import (
	"context"
	"fmt"

	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// TablelandMesa is the main implementation of Tableland spec.
type TablelandMesa struct {
	parser      parsing.SQLValidator
	userStore   sqlstore.UserStore
	chainStacks map[tableland.ChainID]chains.ChainStack
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(
	parser parsing.SQLValidator,
	userStore sqlstore.UserStore,
	chainStacks map[tableland.ChainID]chains.ChainStack,
) tableland.Tableland {
	return &TablelandMesa{
		parser:      parser,
		userStore:   userStore,
		chainStacks: chainStacks,
	}
}

// RunReadQuery allows the user to run SQL.
func (t *TablelandMesa) RunReadQuery(ctx context.Context, statement string) (*tableland.TableData, error) {
	readStmt, err := t.parser.ValidateReadQuery(statement)
	if err != nil {
		return nil, fmt.Errorf("validating query: %s", err)
	}

	queryResult, err := t.runSelect(ctx, readStmt)
	if err != nil {
		return nil, fmt.Errorf("running read statement: %s", err)
	}
	return queryResult, nil
}

func (t *TablelandMesa) runSelect(
	ctx context.Context,
	stmt parsing.ReadStmt,
) (*tableland.TableData, error) {
	queryResult, err := t.userStore.Read(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("executing read-query: %s", err)
	}

	return queryResult, nil
}
