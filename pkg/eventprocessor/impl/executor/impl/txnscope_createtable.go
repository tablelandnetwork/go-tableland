package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func (ts *txnScope) executeCreateTableEvent(
	ctx context.Context,
	be eventfeed.TxnEvents,
	e *ethereum.ContractCreateTable,
) (executor.TxnExecutionResult, error) {
	createStmt, err := ts.parser.ValidateCreateTable(e.Statement, ts.chainID)
	if err != nil {
		err := fmt.Sprintf("query validation: %s", err)
		return executor.TxnExecutionResult{Error: &err}, nil
	}

	if e.TableId == nil {
		return executor.TxnExecutionResult{Error: &tableIDIsEmpty}, nil
	}
	tableID := tableland.TableID(*e.TableId)

	if err := ts.insertTable(ctx, tableID, e.Owner.Hex(), createStmt); err != nil {
		var dbErr *executor.ErrQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("table creation execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			return executor.TxnExecutionResult{Error: &err}, nil
		}
		return executor.TxnExecutionResult{}, fmt.Errorf("executing table creation: %s", err)
	}

	return executor.TxnExecutionResult{TableID: &tableID}, nil
}

// insertTable creates a new table in Tableland:
// - Registers the table in the system-wide table registry.
// - Executes the CREATE statement.
// - Add default privileges in the system_acl table.
func (ts *txnScope) insertTable(
	ctx context.Context,
	id tableland.TableID,
	controller string,
	createStmt parsing.CreateStmt,
) error {
	if _, err := ts.txn.ExecContext(ctx,
		`INSERT INTO registry ("chain_id", "id","controller","prefix","structure") 
		  	 VALUES (?1,?2,?3,?4,?5);`,
		ts.chainID,
		id.String(),
		controller,
		createStmt.GetPrefix(),
		createStmt.GetStructureHash()); err != nil {
		return fmt.Errorf("inserting new table in system-wide registry: %s", err)
	}

	if _, err := ts.txn.ExecContext(ctx,
		`INSERT INTO system_acl ("chain_id","table_id","controller","privileges") 
			 VALUES (?1,?2,?3,?4);`,
		ts.chainID,
		id.String(),
		controller,
		tableland.PrivInsert.Bitfield|tableland.PrivUpdate.Bitfield|tableland.PrivDelete.Bitfield,
	); err != nil {
		return fmt.Errorf("inserting new entry into system acl: %s", err)
	}

	query, err := createStmt.GetRawQueryForTableID(id)
	if err != nil {
		return fmt.Errorf("get query for table id: %s", err)
	}
	if _, err := ts.txn.ExecContext(ctx, query); err != nil {
		return fmt.Errorf("exec CREATE statement: %s", err)
	}

	return nil
}
