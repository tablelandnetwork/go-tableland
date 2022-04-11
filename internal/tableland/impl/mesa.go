package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
	"github.com/textileio/go-tableland/pkg/txn"
)

var log = logger.With().Str("component", "mesa").Logger()

// TablelandMesa is the main implementation of Tableland spec.
type TablelandMesa struct {
	store    sqlstore.SQLStore
	txnp     txn.TxnProcessor
	parser   parsing.SQLValidator
	acl      tableland.ACL
	registry tableregistry.TableRegistry
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(
	store sqlstore.SQLStore,
	parser parsing.SQLValidator,
	txnp txn.TxnProcessor,
	acl tableland.ACL,
	registry tableregistry.TableRegistry) tableland.Tableland {
	return &TablelandMesa{
		store:    store,
		acl:      acl,
		parser:   parser,
		txnp:     txnp,
		registry: registry,
	}
}

// CreateTable allows the user to create a table.
func (t *TablelandMesa) CreateTable(
	ctx context.Context,
	req tableland.CreateTableRequest) (tableland.CreateTableResponse, error) {
	controller := common.HexToAddress(req.Controller)

	if err := t.acl.CheckAuthorization(ctx, controller); err != nil {
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

	fullTableName := fmt.Sprintf("%s_%s", createStmt.GetNamePrefix(), req.ID)
	if req.DryRun {
		return tableland.CreateTableResponse{Name: fullTableName}, nil
	}

	isOwner, err := t.acl.IsOwner(ctx, controller, tableID)
	if err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("failed to check owner: %s", err)
	}
	if !isOwner {
		return tableland.CreateTableResponse{}, errors.New("you aren't the owner of the provided token")
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
	if err := b.InsertTable(ctx, tableID, controller.Hex(), req.Description, createStmt); err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("processing table registration: %s", err)
	}
	if err := b.Commit(ctx); err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("committing changes: %s", err)
	}

	if err := t.store.IncrementCreateTableCount(ctx, controller.Hex()); err != nil {
		log.Error().Err(err).Msg("incrementing create table count")
	}

	return tableland.CreateTableResponse{
		Name:          fullTableName,
		StructureHash: createStmt.GetStructureHash(),
	}, nil
}

// CalculateTableHash allows to calculate the structure hash for a CREATE TABLE statement.
// This RPC method is stateless.
func (t *TablelandMesa) CalculateTableHash(
	ctx context.Context,
	req tableland.CalculateTableHashRequest) (tableland.CalculateTableHashResponse, error) {
	createStmt, err := t.parser.ValidateCreateTable(req.CreateStatement)
	if err != nil {
		return tableland.CalculateTableHashResponse{}, fmt.Errorf("create stmt validation: %s", err)
	}
	return tableland.CalculateTableHashResponse{
		StructureHash: createStmt.GetStructureHash(),
	}, nil
}

// RunSQL allows the user to run SQL.
func (t *TablelandMesa) RunSQL(ctx context.Context, req tableland.RunSQLRequest) (tableland.RunSQLResponse, error) {
	readStmt, mutatingStmts, err := t.parser.ValidateRunSQL(req.Statement)
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

	// Mutating statements
	tableID := mutatingStmts[0].GetTableID()
	tx, err := t.registry.RunSQL(ctx, common.HexToAddress(req.Controller), tableID, req.Statement)
	if err != nil {
		return tableland.RunSQLResponse{}, fmt.Errorf("sending tx: %s", err)
	}

	response := tableland.RunSQLResponse{}
	response.Transaction.Hash = tx.Hash().String()
	t.incrementRunSQLCount(ctx, req.Controller)
	return response, nil
}

// Authorize is a convenience API giving the client something to call to trigger authorization.
func (t *TablelandMesa) Authorize(ctx context.Context, req tableland.AuthorizeRequest) error {
	if err := t.acl.CheckAuthorization(ctx, common.HexToAddress(req.Controller)); err != nil {
		return fmt.Errorf("checking address authorization: %s", err)
	}
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

func (t *TablelandMesa) incrementRunSQLCount(ctx context.Context, address string) {
	if err := t.store.IncrementRunSQLCount(ctx, address); err != nil {
		log.Error().Err(err).Msg("incrementing run sql count")
	}
}
