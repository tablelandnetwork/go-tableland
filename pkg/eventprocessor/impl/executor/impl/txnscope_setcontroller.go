package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func (ts *txnScope) executeSetControllerEvent(
	ctx context.Context,
	be eventfeed.TxnEvents,
	e *ethereum.ContractSetController,
) (executor.TxnExecutionResult, error) {
	if e.TableId == nil {
		return executor.TxnExecutionResult{Error: &tableIDIsEmpty}, nil
	}
	tableID := tableland.TableID(*e.TableId)

	if err := ts.setController(ctx, tableID, e.Controller); err != nil {
		var dbErr *executor.ErrQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("set controller execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			return executor.TxnExecutionResult{Error: &err}, nil
		}
		return executor.TxnExecutionResult{}, fmt.Errorf("executing set controller: %s", err)
	}

	return executor.TxnExecutionResult{TableID: &tableID}, nil
}

// SetController sets and unsets the controller of a table.
func (ts *txnScope) setController(
	ctx context.Context,
	id tableland.TableID,
	controller common.Address,
) error {
	if controller == common.HexToAddress("0x0") {
		if _, err := ts.txn.ExecContext(ctx,
			`DELETE FROM system_controller WHERE chain_id = ?1 AND table_id = ?2;`,
			ts.chainID,
			id.String(),
		); err != nil {
			if code, ok := isErrCausedByQuery(err); ok {
				return &executor.ErrQueryExecution{
					Code: "SQLITE_" + code,
					Msg:  err.Error(),
				}
			}
			return fmt.Errorf("deleting entry from system controller: %s", err)
		}
	} else {
		if _, err := ts.txn.ExecContext(ctx,
			`INSERT INTO system_controller ("chain_id", "table_id", "controller") 
				VALUES (?1, ?2, ?3)
				ON CONFLICT ("chain_id", "table_id")
				DO UPDATE set controller = ?3;`,
			ts.chainID,
			id.String(),
			controller.Hex(),
		); err != nil {
			if code, ok := isErrCausedByQuery(err); ok {
				return &executor.ErrQueryExecution{
					Code: "SQLITE_" + code,
					Msg:  err.Error(),
				}
			}
			return fmt.Errorf("inserting new entry into system controller: %s", err)
		}
	}
	return nil
}
