package impl

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/rs/zerolog"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

var tableIDIsEmpty = "table id is empty"

type txnScope struct {
	log zerolog.Logger

	parser  parsing.SQLValidator
	acl     tableland.ACL
	chainID tableland.ChainID

	txn *sql.Tx
}

func (ts *txnScope) executeTxnEvents(ctx context.Context, tx *sql.Tx, evmTxn eventfeed.TxnEvents) (tableland.TableID, error) {
	var receipt eventprocessor.Receipt
	var err error

	for _, e := range evmTxn.Events {
		switch e := e.(type) {
		case *ethereum.ContractRunSQL:
			ts.log.Debug().Str("statement", e.Statement).Msgf("executing run-sql event")
			receipt, err = ts.executeRunSQLEvent(ctx, evmTxn, e)
			if err != nil {
				return tableland.TableID{}, fmt.Errorf("executing runsql event: %s", err)
			}
		case *ethereum.ContractCreateTable:
			ts.log.Debug().
				Str("owner", e.Owner.Hex()).
				Str("tokenId", e.TableId.String()).
				Str("statement", e.Statement).
				Msgf("executing create-table event")
			receipt, err = ts.executeCreateTableEvent(ctx, evmTxn, e)
			if err != nil {
				return tableland.TableID{}, fmt.Errorf("executing create-table event: %s", err)
			}
		case *ethereum.ContractSetController:
			ts.log.Debug().
				Str("controller", e.Controller.Hex()).
				Str("tokenId", e.TableId.String()).
				Msgf("executing set-controller event")
			receipt, err = ts.executeSetControllerEvent(ctx, evmTxn, e)
			if err != nil {
				return tableland.TableID{}, fmt.Errorf("executing set-controller event: %s", err)
			}
		case *ethereum.ContractTransferTable:
			ts.log.Debug().
				Str("from", e.From.Hex()).
				Str("to", e.To.Hex()).
				Str("tableId", e.TableId.String()).
				Msgf("executing table transfer event")

			receipt, err = ts.executeTransferEvent(ctx, evmTxn, e)
			if err != nil {
				return tableland.TableID{}, fmt.Errorf("executing transfer event: %s", err)
			}
		default:
			return tableland.TableID{}, fmt.Errorf("unknown event type %t", e)
		}

		// If the current event fail, we stop processing further events in this transaction and already
		// return the failed receipt. This receipt contains the index of this failed event.
		if receipt.Error != nil {
			return receipt, nil
		}
	}

	return receipt, nil
}
