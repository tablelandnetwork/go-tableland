package impl

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
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
	registry tableregistry.TableRegistry
	parser   parsing.SQLValidator
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
		txnp:     txnp,
		parser:   parser,
	}
}

// CreateTable allows the user to create a table.
func (t *TablelandMesa) CreateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	if err := t.authorize(ctx, req.Controller); err != nil {
		return tableland.Response{}, fmt.Errorf("checking address authorization: %s", err)
	}

	uuid, err := uuid.Parse(req.TableID)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("failed to parse uuid: %s", err)
	}

	if err := t.parser.ValidateCreateTable(req.Statement); err != nil {
		return tableland.Response{}, fmt.Errorf("query validation: %s", err)
	}

	b, err := t.txnp.OpenBatch(ctx)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("opening batch: %s", err)
	}
	defer func() {
		if err := b.Close(ctx); err != nil {
			log.Error().Err(err).Msg("closing batch")
		}
	}()

	if err := b.InsertTable(ctx, uuid, req.Controller, req.Type, req.Statement); err != nil {
		return tableland.Response{}, fmt.Errorf("processing table registration: %s", err)
	}
	if err := b.Commit(ctx); err != nil {
		return tableland.Response{}, fmt.Errorf("committing changes: %s", err)
	}

	if err := t.store.IncrementCreateTableCount(ctx, req.Controller); err != nil {
		log.Error().Err(err).Msg("incrementing create table count")
	}

	return tableland.Response{Message: "Table created"}, nil
}

// UpdateTable allows the user to update a table.
func (t *TablelandMesa) UpdateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	// this is not going to be implemented
	return tableland.Response{Message: "Table updated"}, nil
}

// RunSQL allows the user to run SQL.
func (t *TablelandMesa) RunSQL(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	uuid, err := uuid.Parse(req.TableID)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("failed to parse uuid: %s", err)
	}

	queryType, writeStmts, err := t.parser.ValidateRunSQL(req.Statement)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("validating query: %s", err)
	}

	switch queryType {
	case parsing.ReadQuery:
		return t.runSelect(ctx, req)
	case parsing.WriteQuery:
		isOwner, err := t.isOwner(ctx, req.Controller, uuid)
		if err != nil {
			return tableland.Response{}, fmt.Errorf("failed to check authorization: %s", err)
		}

		if !isOwner {
			return tableland.Response{}, errors.New("you aren't authorized")
		}
		return t.runInsertOrUpdate(ctx, writeStmts)
	}

	return tableland.Response{}, errors.New("invalid command")
}

// Authorize is a convenience API giving the client something to call to trigger authorization.
func (t *TablelandMesa) Authorize(ctx context.Context, req tableland.Request) error {
	if err := t.authorize(ctx, req.Controller); err != nil {
		return fmt.Errorf("checking address authorization: %s", err)
	}
	return nil
}

func (t *TablelandMesa) runInsertOrUpdate(ctx context.Context, ws []parsing.WriteStmt) (tableland.Response, error) {
	b, err := t.txnp.OpenBatch(ctx)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("opening batch: %s", err)
	}
	defer func() {
		if err := b.Close(ctx); err != nil {
			log.Error().Err(err).Msg("closing batch")
		}
	}()
	if err := b.ExecWriteQueries(ctx, ws); err != nil {
		return tableland.Response{}, fmt.Errorf("executing write-query: %s", err)
	}

	if err := b.Commit(ctx); err != nil {
		return tableland.Response{}, fmt.Errorf("committing changes: %s", err)
	}

	t.incrementRunSQLCount(ctx, req.Controller)

	return tableland.Response{Message: "Command executed"}, nil
}

func (t *TablelandMesa) runSelect(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	data, err := t.store.Read(ctx, req.Statement)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("executing read-query: %s", err)
	}

	t.incrementRunSQLCount(ctx, req.Controller)

	return tableland.Response{Message: "Select executed", Data: data}, nil
}

func (t *TablelandMesa) isOwner(ctx context.Context, controller string, table uuid.UUID) (bool, error) {
	isOwner, err := t.registry.IsOwner(ctx, common.HexToAddress(controller), t.uuidToBigInt(table))
	if err != nil {
		return false, fmt.Errorf("failed to execute contract call: %s", err)
	}
	return isOwner, nil
}

func (t *TablelandMesa) uuidToBigInt(uuid uuid.UUID) *big.Int {
	var n big.Int
	n.SetString(strings.Replace(uuid.String(), "-", "", 4), 16)
	return &n
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
