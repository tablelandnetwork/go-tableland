package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/cmd/api/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

var log = logger.With().Str("component", "mesa").Logger()

// TablelandMesa is the main implementation of Tableland spec.
type TablelandMesa struct {
	store           sqlstore.SQLStore
	parser          parsing.SQLValidator
	chainRegistries map[tableland.ChainID]tableregistry.TableRegistry
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(
	store sqlstore.SQLStore,
	parser parsing.SQLValidator,
	chainRegistries map[tableland.ChainID]tableregistry.TableRegistry) tableland.Tableland {
	return &TablelandMesa{
		store:           store,
		parser:          parser,
		chainRegistries: chainRegistries,
	}
}

// ValidateCreateTable allows to validate a CREATE TABLE statement and also return the structure hash of it.
// This RPC method is stateless.
func (t *TablelandMesa) ValidateCreateTable(
	ctx context.Context,
	req tableland.ValidateCreateTableRequest) (tableland.ValidateCreateTableResponse, error) {
	createStmt, err := t.parser.ValidateCreateTable(req.CreateStatement)
	if err != nil {
		return tableland.ValidateCreateTableResponse{}, fmt.Errorf("parsing create table statement: %s", err)
	}
	return tableland.ValidateCreateTableResponse{StructureHash: createStmt.GetStructureHash()}, nil
}

// RunSQL allows the user to run SQL.
func (t *TablelandMesa) RunSQL(ctx context.Context, req tableland.RunSQLRequest) (tableland.RunSQLResponse, error) {
	ctxController := ctx.Value(middlewares.ContextKeyAddress)
	controller, ok := ctxController.(string)
	if !ok || controller == "" {
		return tableland.RunSQLResponse{}, errors.New("no controller address found in context")
	}
	readStmt, mutatingStmts, err := t.parser.ValidateRunSQL(req.Statement)
	if err != nil {
		return tableland.RunSQLResponse{}, fmt.Errorf("validating query: %s", err)
	}

	// Read statement
	if readStmt != nil {
		queryResult, err := t.runSelect(ctx, controller, readStmt)
		if err != nil {
			return tableland.RunSQLResponse{}, fmt.Errorf("running read statement: %s", err)
		}
		return tableland.RunSQLResponse{Result: queryResult}, nil
	}

	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.RunSQLResponse{}, errors.New("no chain id found in context")
	}
	registry, ok := t.chainRegistries[chainID]
	if !ok {
		return tableland.RunSQLResponse{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}

	// Mutating statements
	tableID := mutatingStmts[0].GetTableID()
	tx, err := registry.RunSQL(ctx, common.HexToAddress(controller), tableID, req.Statement)
	if err != nil {
		return tableland.RunSQLResponse{}, fmt.Errorf("sending tx: %s", err)
	}

	response := tableland.RunSQLResponse{}
	response.Transaction.Hash = tx.Hash().String()
	t.incrementRunSQLCount(ctx, controller)
	return response, nil
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (t *TablelandMesa) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest) (tableland.GetReceiptResponse, error) {
	if err := (&common.Hash{}).UnmarshalText([]byte(req.TxnHash)); err != nil {
		return tableland.GetReceiptResponse{}, fmt.Errorf("invalid txn hash: %s", err)
	}

	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.GetReceiptResponse{}, errors.New("no chain id found in context")
	}
	receipt, ok, err := t.store.GetReceipt(ctx, chainID, req.TxnHash)
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

// SetController allows users to the controller for a token id.
func (t *TablelandMesa) SetController(
	ctx context.Context,
	req tableland.SetControllerRequest) (tableland.SetControllerResponse, error) {
	tableID, err := tableland.NewTableID(req.TokenID)
	if err != nil {
		return tableland.SetControllerResponse{}, fmt.Errorf("parsing table id: %s", err)
	}

	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.SetControllerResponse{}, errors.New("no chain id found in context")
	}
	registry, ok := t.chainRegistries[chainID]
	if !ok {
		return tableland.SetControllerResponse{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}

	tx, err := registry.SetController(ctx, common.HexToAddress(req.Caller), tableID, common.HexToAddress(req.Controller))
	if err != nil {
		return tableland.SetControllerResponse{}, fmt.Errorf("sending tx: %s", err)
	}

	response := tableland.SetControllerResponse{}
	response.Transaction.Hash = tx.Hash().String()
	t.incrementRunSQLCount(ctx, req.Controller)
	return response, nil
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
