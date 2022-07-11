package impl

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/router/middlewares"
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

// ValidateCreateTable allows to validate a CREATE TABLE statement and also return the structure hash of it.
// This RPC method is stateless.
func (t *TablelandMesa) ValidateCreateTable(
	ctx context.Context,
	req tableland.ValidateCreateTableRequest,
) (tableland.ValidateCreateTableResponse, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.ValidateCreateTableResponse{}, errors.New("no chain id found in context")
	}
	createStmt, err := t.parser.ValidateCreateTable(req.CreateStatement, chainID)
	if err != nil {
		return tableland.ValidateCreateTableResponse{}, fmt.Errorf("parsing create table statement: %s", err)
	}
	return tableland.ValidateCreateTableResponse{StructureHash: createStmt.GetStructureHash()}, nil
}

// RelayWriteQuery allows the user to rely on the validator wrapping the query in a chain transaction.
func (t *TablelandMesa) RelayWriteQuery(
	ctx context.Context,
	req tableland.RelayWriteQueryRequest,
) (tableland.RelayWriteQueryResponse, error) {
	ctxController := ctx.Value(middlewares.ContextKeyAddress)
	controller, ok := ctxController.(string)
	if !ok || controller == "" {
		return tableland.RelayWriteQueryResponse{}, errors.New("no controller address found in context")
	}
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.RelayWriteQueryResponse{}, errors.New("no chain id found in context")
	}
	stack, ok := t.chainStacks[chainID]
	if !ok {
		return tableland.RelayWriteQueryResponse{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}

	mutatingStmts, err := t.parser.ValidateMutatingQuery(req.Statement, chainID)
	if err != nil {
		return tableland.RelayWriteQueryResponse{}, fmt.Errorf("validating query: %s", err)
	}

	tableID := mutatingStmts[0].GetTableID()
	tx, err := stack.Registry.RunSQL(ctx, common.HexToAddress(controller), tableID, req.Statement)
	if err != nil {
		return tableland.RelayWriteQueryResponse{}, fmt.Errorf("sending tx: %s", err)
	}

	response := tableland.RelayWriteQueryResponse{}
	response.Transaction.Hash = tx.Hash().String()
	return response, nil
}

// RunReadQuery allows the user to run SQL.
func (t *TablelandMesa) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest,
) (tableland.RunReadQueryResponse, error) {
	readStmt, err := t.parser.ValidateReadQuery(req.Statement)
	if err != nil {
		return tableland.RunReadQueryResponse{}, fmt.Errorf("validating query: %s", err)
	}

	queryResult, err := t.runSelect(ctx, readStmt, req.JSONStrings)
	if err != nil {
		return tableland.RunReadQueryResponse{}, fmt.Errorf("running read statement: %s", err)
	}

	count := len(queryResult.Rows)

	if req.Output == tableland.Table {
		return tableland.RunReadQueryResponse{Result: queryResult, RowCount: count}, nil
	}

	objects := toObjects(queryResult)
	if req.Extract {
		objects, err = extract(objects)
		if err != nil {
			return tableland.RunReadQueryResponse{}, fmt.Errorf("extracting values: %s", err)
		}
	}

	if !req.Unwrap {
		return tableland.RunReadQueryResponse{Result: objects, RowCount: count}, nil
	}

	unwrapped, err := unwrap(objects)
	if err != nil {
		return tableland.RunReadQueryResponse{}, fmt.Errorf("unwrapping values: %s", err)
	}

	var res interface{} = unwrapped
	if ctx.Value(tableland.UserControllerContextKey) == nil && count > 1 {
		// This is a JSON RPC request so we can't embed multiline
		// jsonl without encoding it as base64.
		res = base64.StdEncoding.EncodeToString(unwrapped)
	}

	return tableland.RunReadQueryResponse{Result: res, RowCount: count}, nil
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (t *TablelandMesa) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest,
) (tableland.GetReceiptResponse, error) {
	if err := (&common.Hash{}).UnmarshalText([]byte(req.TxnHash)); err != nil {
		return tableland.GetReceiptResponse{}, fmt.Errorf("invalid txn hash: %s", err)
	}

	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.GetReceiptResponse{}, errors.New("no chain id found in context")
	}
	stack, ok := t.chainStacks[chainID]
	if !ok {
		return tableland.GetReceiptResponse{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	receipt, ok, err := stack.Store.GetReceipt(ctx, req.TxnHash)
	if err != nil {
		return tableland.GetReceiptResponse{}, fmt.Errorf("get txn receipt: %s", err)
	}
	if !ok {
		return tableland.GetReceiptResponse{Ok: false}, nil
	}

	ret := tableland.GetReceiptResponse{
		Ok: ok,
		Receipt: &tableland.TxnReceipt{
			ChainID:     receipt.ChainID,
			TxnHash:     receipt.TxnHash,
			BlockNumber: receipt.BlockNumber,
			Error:       receipt.Error,
		},
	}
	if receipt.TableID != nil {
		tID := receipt.TableID.String()
		ret.Receipt.TableID = &tID
	}

	return ret, nil
}

// SetController allows users to the controller for a token id.
func (t *TablelandMesa) SetController(
	ctx context.Context,
	req tableland.SetControllerRequest,
) (tableland.SetControllerResponse, error) {
	ctxCaller := ctx.Value(middlewares.ContextKeyAddress)
	caller, ok := ctxCaller.(string)
	if !ok || caller == "" {
		return tableland.SetControllerResponse{}, errors.New("no caller address found in context")
	}
	tableID, err := tableland.NewTableID(req.TokenID)
	if err != nil {
		return tableland.SetControllerResponse{}, fmt.Errorf("parsing table id: %s", err)
	}

	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return tableland.SetControllerResponse{}, errors.New("no chain id found in context")
	}
	stack, ok := t.chainStacks[chainID]
	if !ok {
		return tableland.SetControllerResponse{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}

	tx, err := stack.Registry.SetController(
		ctx, common.HexToAddress(caller), tableID, common.HexToAddress(req.Controller))
	if err != nil {
		return tableland.SetControllerResponse{}, fmt.Errorf("sending tx: %s", err)
	}

	response := tableland.SetControllerResponse{}
	response.Transaction.Hash = tx.Hash().String()
	return response, nil
}

func (t *TablelandMesa) runSelect(
	ctx context.Context,
	stmt parsing.ReadStmt,
	jsonStrings bool,
) (*sqlstore.UserRows, error) {
	queryResult, err := t.userStore.Read(ctx, stmt, jsonStrings)
	if err != nil {
		return nil, fmt.Errorf("executing read-query: %s", err)
	}

	return queryResult, nil
}

func toObjects(in *sqlstore.UserRows) []interface{} {
	objects := make([]interface{}, len(in.Rows))
	for i, row := range in.Rows {
		object := make(map[string]interface{}, len(row))
		for j, val := range row {
			object[in.Columns[j].Name] = val
		}
		objects[i] = object
	}
	return objects
}

func extract(in []interface{}) ([]interface{}, error) {
	extracted := make([]interface{}, len(in))
	for i, item := range in {
		object := item.(map[string]interface{})
		if len(object) != 1 {
			return nil, fmt.Errorf("can only extract values for result sets with one column but this has %d", len(object))
		}
		for _, val := range object {
			extracted[i] = val
			break
		}
	}
	return extracted, nil
}

func unwrap(in []interface{}) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	for i, item := range in {
		if i != 0 {
			_, _ = buf.Write([]byte("\n"))
		}
		b, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(b)
	}
	return buf.Bytes(), nil
}
