package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func (ts *txnScope) executeSetControllerEvent(
	ctx context.Context,
	be eventfeed.TxnEvents,
	e *ethereum.ContractSetController,
) (eventprocessor.Receipt, error) {
	receipt := eventprocessor.Receipt{
		ChainID:      ts.chainID,
		BlockNumber:  ts.blockNumber,
		IndexInBlock: ts.idxInBlock,
		TxnHash:      be.TxnHash.String(),
	}

	if e.TableId == nil {
		receipt.Error = &tableIDIsEmpty
		return receipt, nil
	}
	tableID := tableland.TableID(*e.TableId)

	if err := ts.setController(ctx, tableID, e.Controller); err != nil {
		var dbErr *executor.ErrQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("set controller execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return tableland.TableID{}, fmt.Errorf("executing set controller: %s", err)
	}

	receipt.TableID = &tableID

	return receipt, nil
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
