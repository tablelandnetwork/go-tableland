package impl

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/chains"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
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
	_ context.Context,
	chainID tableland.ChainID,
	statement string,
) (string, error) {
	createStmt, err := t.parser.ValidateCreateTable(statement, chainID)
	if err != nil {
		return "", fmt.Errorf("parsing create table statement: %s", err)
	}
	return createStmt.GetStructureHash(), nil
}

// ValidateWriteQuery allows the user to validate a write query.
func (t *TablelandMesa) ValidateWriteQuery(
	ctx context.Context,
	chainID tableland.ChainID,
	statement string,
) (tables.TableID, error) {
	stack, chainOk := t.chainStacks[chainID]
	if !chainOk {
		return tables.TableID{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}

	mutatingStmts, err := t.parser.ValidateMutatingQuery(statement, chainID)
	if err != nil {
		return tables.TableID{}, fmt.Errorf("validating query: %s", err)
	}

	tableID := mutatingStmts[0].GetTableID()

	table, err := stack.Store.GetTable(ctx, tableID)
	// if the tableID is not valid err will exist
	if err != nil {
		return tables.TableID{}, fmt.Errorf("getting table: %s", err)
	}
	// if the prefix is wrong the statement is not valid
	prefix := mutatingStmts[0].GetPrefix()
	if table.Prefix != prefix {
		return tables.TableID{}, fmt.Errorf(
			"table prefix doesn't match (exp %s, got %s)", table.Prefix, prefix)
	}

	return tableID, nil
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

// GetReceipt returns the receipt of a processed event by txn hash.
func (t *TablelandMesa) GetReceipt(
	ctx context.Context,
	chainID tableland.ChainID,
	txnHash string,
) (bool, *tableland.TxnReceipt, error) {
	if err := (&common.Hash{}).UnmarshalText([]byte(txnHash)); err != nil {
		return false, nil, fmt.Errorf("invalid txn hash: %s", err)
	}
	stack, ok := t.chainStacks[chainID]
	if !ok {
		return false, nil, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	receipt, ok, err := stack.Store.GetReceipt(ctx, txnHash)
	if err != nil {
		return false, nil, fmt.Errorf("get txn receipt: %s", err)
	}
	if !ok {
		return false, nil, nil
	}

	errorEventIdx := -1
	if receipt.ErrorEventIdx != nil {
		errorEventIdx = *receipt.ErrorEventIdx
	}
	errorMsg := ""
	if receipt.Error != nil {
		errorMsg = *receipt.Error
	}

	ret := &tableland.TxnReceipt{
		ChainID:       receipt.ChainID,
		TxnHash:       receipt.TxnHash,
		BlockNumber:   receipt.BlockNumber,
		Error:         errorMsg,
		ErrorEventIdx: errorEventIdx,
	}

	if receipt.TableID != nil {
		tID := receipt.TableID.String()
		ret.TableID = &tID
	}

	return ok, ret, nil
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
