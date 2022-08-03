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

func (ts *txnScope) executeTransferEvent(
	ctx context.Context,
	be eventfeed.TxnEvents,
	e *ethereum.ContractTransferTable,
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
	if err := ts.changeTableOwner(ctx, tableID, e.To); err != nil {
		var dbErr *executor.ErrQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("change table owner execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return tableland.TableID{}, fmt.Errorf("executing change table owner: %s", err)
	}

	privileges := tableland.Privileges{tableland.PrivInsert, tableland.PrivUpdate, tableland.PrivDelete}
	if err := ts.executeRevokePrivilegesTx(ctx, tableID, e.From, privileges); err != nil {
		var dbErr *executor.ErrQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("revoke privileges execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return tableland.TableID{}, fmt.Errorf("executing revoke privileges: %s", err)
	}
	if err := ts.executeGrantPrivilegesTx(ctx, tableID, e.To, privileges); err != nil {
		var dbErr *executor.ErrQueryExecution
		if errors.As(err, &dbErr) {
			err := fmt.Sprintf("grant privileges execution failed (code: %s, msg: %s)", dbErr.Code, dbErr.Msg)
			receipt.Error = &err
			return receipt, nil
		}
		return tableland.TableID{}, fmt.Errorf("executing grant privileges: %s", err)
	}

	receipt.TableID = &tableID
	return receipt, nil
}

// changeTableOwner changes the owner of the table in the registry table.
func (ts *txnScope) changeTableOwner(
	ctx context.Context,
	id tableland.TableID,
	newOwner common.Address,
) error {
	if _, err := ts.txn.ExecContext(ctx,
		`UPDATE registry SET controller = ?1 WHERE id = ?2 AND chain_id = ?3;`,
		newOwner.Hex(),
		id.String(),
		ts.chainID,
	); err != nil {
		if code, ok := isErrCausedByQuery(err); ok {
			return &executor.ErrQueryExecution{
				Code: "SQLITE_" + code,
				Msg:  err.Error(),
			}
		}
		return fmt.Errorf("updating table owner: %s", err)
	}

	return nil
}
