package impl

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

var tableIDIsEmpty = "table id is empty"

// errQueryExecution is an error returned when the query execution failed
// with a cause related to th query itself. Retrying the execution of this query
// will always return an error (e.g: inserting a string in an integer column).
// A query execution failure due to the database being down or any other infrastructure
// problem isn't an ErrQueryExecution error.
type errQueryExecution struct {
	Code string
	Msg  string
}

// Error returns a string representation of the query execution error.
func (e *errQueryExecution) Error() string {
	return fmt.Sprintf("query execution failed with code %s: %s", e.Code, e.Msg)
}

type txnScope struct {
	log zerolog.Logger

	parser            parsing.SQLValidator
	statementResolver sqlparser.WriteStatementResolver

	acl       tableland.ACL
	scopeVars scopeVars

	txn *sql.Tx
}

type eventExecutionResult struct {
	TableID *tables.TableID
	Error   *string
}

func (ts *txnScope) executeTxnEvents(
	ctx context.Context,
	evmTxn eventfeed.TxnEvents,
) (executor.TxnExecutionResult, error) {
	var res eventExecutionResult
	var err error

	for idx, event := range evmTxn.Events {
		switch event := event.(type) {
		case *ethereum.ContractRunSQL:
			ts.log.Debug().Str("statement", event.Statement).Msgf("executing run-sql event")
			res, err = ts.executeRunSQLEvent(ctx, event)
			if err != nil {
				return executor.TxnExecutionResult{}, fmt.Errorf("executing runsql event: %s", err)
			}
		case *ethereum.ContractCreateTable:
			ts.log.Debug().
				Str("owner", event.Owner.Hex()).
				Str("token_id", event.TableId.String()).
				Str("statement", event.Statement).
				Msgf("executing create-table event")
			res, err = ts.executeCreateTableEvent(ctx, event)
			if err != nil {
				return executor.TxnExecutionResult{}, fmt.Errorf("executing create-table event: %s", err)
			}
		case *ethereum.ContractSetController:
			ts.log.Debug().
				Str("controller", event.Controller.Hex()).
				Str("token_id", event.TableId.String()).
				Msgf("executing set-controller event")
			res, err = ts.executeSetControllerEvent(ctx, event)
			if err != nil {
				return executor.TxnExecutionResult{}, fmt.Errorf("executing set-controller event: %s", err)
			}
		case *ethereum.ContractTransferTable:
			ts.log.Debug().
				Str("from", event.From.Hex()).
				Str("to", event.To.Hex()).
				Str("tableId", event.TableId.String()).
				Msgf("executing table transfer event")

			res, err = ts.executeTransferEvent(ctx, event)
			if err != nil {
				return executor.TxnExecutionResult{}, fmt.Errorf("executing transfer event: %s", err)
			}
		default:
			return executor.TxnExecutionResult{}, fmt.Errorf("unknown event type %t", event)
		}

		// If the current event fail, we stop processing further events in this transaction and already
		// return the failed receipt. This receipt contains the index of this failed event.
		if res.Error != nil {
			return executor.TxnExecutionResult{
				TableID:       res.TableID,
				Error:         res.Error,
				ErrorEventIdx: &idx,
			}, nil
		}
	}

	return executor.TxnExecutionResult{TableID: res.TableID}, nil
}
