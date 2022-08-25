package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func (ts *txnScope) executeCreateTableEvent(
	ctx context.Context,
	e *ethereum.ContractCreateTable,
) (eventExecutionResult, error) {
	createStmt, err := ts.parser.ValidateCreateTable(e.Statement, ts.scopeVars.ChainID)
	if err != nil {
		err := fmt.Sprintf("query validation: %s", err)
		return eventExecutionResult{Error: &err}, nil
	}

	if e.TableId == nil {
		return eventExecutionResult{Error: &tableIDIsEmpty}, nil
	}
	tableID := tableland.TableID(*e.TableId)

	if err := ts.insertTable(ctx, tableID, e.Owner.Hex(), createStmt); err != nil {
		var dbErr *errQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("table creation execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			return eventExecutionResult{Error: &err}, nil
		}
		return eventExecutionResult{}, fmt.Errorf("executing table creation: %s", err)
	}

	return eventExecutionResult{TableID: &tableID}, nil
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
		ts.scopeVars.ChainID,
		id.String(),
		controller,
		createStmt.GetPrefix(),
		createStmt.GetStructureHash()); err != nil {
		return fmt.Errorf("inserting new table in system-wide registry: %s", err)
	}

	if _, err := ts.txn.ExecContext(ctx,
		`INSERT INTO system_acl ("chain_id","table_id","controller","privileges") 
			 VALUES (?1,?2,?3,?4);`,
		ts.scopeVars.ChainID,
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
		if code, ok := isErrCausedByQuery(err); ok {
			return &errQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("exec CREATE statement: %s", err)
	}

	return nil
}
