package impl

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
	"github.com/textileio/go-tableland/pkg/txn"
)

// TablelandMesa is the main implementation of Tableland spec.
type TablelandMesa struct {
	store    sqlstore.SQLStore
	txnp     txn.TxnProcessor
	parser   parsing.SQLValidator
	registry tableregistry.TableRegistry
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(
	store sqlstore.SQLStore,
	registry tableregistry.TableRegistry,
	parser parsing.SQLValidator,
	txnp txn.TxnProcessor) tableland.Tableland {
	return &TablelandMesa{
		store:    store,
		registry: registry,
		parser:   parser,
		txnp:     txnp,
	}
}

// CreateTable allows the user to create a table.
func (t *TablelandMesa) CreateTable(
	ctx context.Context,
	req tableland.CreateTableRequest) (tableland.CreateTableResponse, error) {
	if err := t.authorize(ctx, req.Controller); err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("checking address authorization: %s", err)
	}
	tableID, err := tableland.NewTableID(req.ID)
	if err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("parsing table id: %s", err)
	}
	if len(req.Description) > 100 {
		return tableland.CreateTableResponse{}, fmt.Errorf("description length should be at most 100")
	}

	createStmt, err := t.parser.ValidateCreateTable(req.Statement)
	if err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("query validation: %s", err)
	}

	isOwner, err := t.isOwner(ctx, req.Controller, tableID.ToBigInt())
	if err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("failed to check authorization: %s", err)
	}
	if !isOwner {
		return tableland.CreateTableResponse{}, errors.New("you aren't authorized")
	}

	b, err := t.txnp.OpenBatch(ctx)
	if err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("opening batch: %s", err)
	}
	defer func() {
		if err := b.Close(ctx); err != nil {
			log.Error().Err(err).Msg("closing batch")
		}
	}()
	if err := b.InsertTable(ctx, tableID, req.Controller, req.Description, createStmt); err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("processing table registration: %s", err)
	}
	if err := b.Commit(ctx); err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("committing changes: %s", err)
	}

	if err := t.store.IncrementCreateTableCount(ctx, req.Controller); err != nil {
		log.Error().Err(err).Msg("incrementing create table count")
	}

	return tableland.CreateTableResponse{
		Tablename: fmt.Sprintf("%s_%s", createStmt.GetNamePrefix(), req.ID),
	}, nil
}

// RunSQL allows the user to run SQL.
func (t *TablelandMesa) RunSQL(ctx context.Context, req tableland.RunSQLRequest) (tableland.RunSQLResponse, error) {
	readStmt, writeStmts, err := t.parser.ValidateRunSQL(req.Statement)
	if err != nil {
		return tableland.RunSQLResponse{}, fmt.Errorf("validating query: %s", err)
	}

	// Read statement
	if readStmt != nil {
		queryResult, err := t.runSelect(ctx, req.Controller, readStmt)
		if err != nil {
			return tableland.RunSQLResponse{}, fmt.Errorf("running read statement: %s", err)
		}
		return tableland.RunSQLResponse{Result: queryResult}, nil
	}

	// Write statements
	tableID := writeStmts[0].GetTableID()
	isOwner, err := t.isOwner(ctx, req.Controller, tableID.ToBigInt())
	if err != nil {
		return tableland.RunSQLResponse{}, fmt.Errorf("failed to check authorization: %s", err)
	}
	if !isOwner {
		return tableland.RunSQLResponse{}, errors.New("you aren't authorized")
	}
	if err := t.runInsertOrUpdate(ctx, req.Controller, writeStmts); err != nil {
		return tableland.RunSQLResponse{}, fmt.Errorf("running statement: %s", err)
	}
	return tableland.RunSQLResponse{}, nil
}

// Authorize is a convenience API giving the client something to call to trigger authorization.
func (t *TablelandMesa) Authorize(ctx context.Context, req tableland.AuthorizeRequest) error {
	if err := t.authorize(ctx, req.Controller); err != nil {
		return fmt.Errorf("checking address authorization: %s", err)
	}
	return nil
}

func (t *TablelandMesa) runInsertOrUpdate(
	ctx context.Context,
	controller string,
	ws []parsing.SugaredWriteStmt) error {
	b, err := t.txnp.OpenBatch(ctx)
	if err != nil {
		return fmt.Errorf("opening batch: %s", err)
	}
	defer func() {
		if err := b.Close(ctx); err != nil {
			log.Error().Err(err).Msg("closing batch")
		}
	}()
	if err := b.ExecWriteQueries(ctx, ws); err != nil {
		return fmt.Errorf("executing write-query: %s", err)
	}

	if err := b.Commit(ctx); err != nil {
		return fmt.Errorf("committing changes: %s", err)
	}

	t.incrementRunSQLCount(ctx, controller)

	return nil
}

func (t *TablelandMesa) runSelect(ctx context.Context, ctrl string, stmt parsing.SugaredReadStmt) (interface{}, error) {
	queryResult, err := t.store.Read(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("executing read-query: %s", err)
	}

	t.incrementRunSQLCount(ctx, ctrl)

	return queryResult, nil
}

func (t *TablelandMesa) isOwner(ctx context.Context, controller string, id *big.Int) (bool, error) {
	isOwner, err := t.registry.IsOwner(ctx, common.HexToAddress(controller), id)
	if err != nil {
		return false, fmt.Errorf("failed to execute contract call: %s", err)
	}
	return isOwner, nil
}

func (t *TablelandMesa) authorize(ctx context.Context, address string) error {
	res, err := t.store.IsAuthorized(ctx, address)
	if err != nil {
		return err
	}

	if !res.IsAuthorized {
		return fmt.Errorf("address not authorized")
	}

	return nil
}

func (t *TablelandMesa) incrementRunSQLCount(ctx context.Context, address string) {
	if err := t.store.IncrementRunSQLCount(ctx, address); err != nil {
		log.Error().Err(err).Msg("incrementing run sql count")
	}
}
