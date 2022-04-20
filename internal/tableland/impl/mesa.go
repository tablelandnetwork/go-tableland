package impl

import (
	"context"
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
	chainID  int64
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(
	store sqlstore.SQLStore,
	parser parsing.SQLValidator,
	txnp txn.TxnProcessor,
	acl tableland.ACL,
	registry tableregistry.TableRegistry,
	chainID int64) tableland.Tableland {
	return &TablelandMesa{
		store:    store,
		acl:      acl,
		parser:   parser,
		txnp:     txnp,
		registry: registry,
		chainID:  chainID,
	}
}

// CreateTable allows the user to validate a CREATE TABLE query.
func (t *TablelandMesa) CreateTable(
	ctx context.Context,
	req tableland.CreateTableRequest) (tableland.CreateTableResponse, error) {
	createStmt, err := t.parser.ValidateCreateTable(req.Statement)
	if err != nil {
		return tableland.CreateTableResponse{}, fmt.Errorf("query validation: %s", err)
	}
	return tableland.CreateTableResponse{StructureHash: createStmt.GetStructureHash()}, nil
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

// GetReceipt returns the receipt of a processed event by txn hash.
func (t *TablelandMesa) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest) (tableland.GetReceiptResponse, error) {
	if err := (&common.Hash{}).UnmarshalText([]byte(req.TxnHash)); err != nil {
		return tableland.GetReceiptResponse{}, fmt.Errorf("invalid txn hash: %s", err)
	}

	receipt, ok, err := t.store.GetReceipt(ctx, t.chainID, req.TxnHash)
	if err != nil {
		return tableland.GetReceiptResponse{}, fmt.Errorf("get txn receipt: %s", err)
	}
	if !ok {
		return tableland.GetReceiptResponse{Ok: false}, nil
	}
	return tableland.GetReceiptResponse{
		Ok: ok,
		Receipt: &tableland.TxnReceipt{
			ChainID:     receipt.ChainID,
			TxnHash:     receipt.TxnHash,
			BlockNumber: receipt.BlockNumber,
			Error:       receipt.Error,
			TableID:     receipt.TableID,
		},
	}, nil
}

// TODO(jsign): waiting for decision to deprecate/delete.
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
