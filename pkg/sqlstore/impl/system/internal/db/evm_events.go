package db

import (
	"context"
	"database/sql"
)

const insertEVMEvent = `
INSERT INTO system_evm_events ("chain_id", "event_json", "address", "topics", "data", "block_number", "tx_hash", "tx_index", "block_hash", "event_index")
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)
`

type InsertEVMEventParams struct {
	ChainID     uint64
	EventJSON   []byte
	Address     string
	Topics      []byte
	Data        []byte
	BlockNumber uint64
	TxHash      string
	TxIndex     uint
	BlockHash   string
	Index       uint
}

func (q *Queries) InsertEVMEvent(ctx context.Context, arg InsertEVMEventParams) error {
	_, err := q.exec(ctx, q.insertEVMEventStmt, insertEVMEvent,
		arg.ChainID,
		arg.EventJSON,
		arg.Address,
		arg.Topics,
		arg.Data,
		arg.BlockNumber,
		arg.TxHash,
		arg.TxIndex,
		arg.BlockHash,
		arg.Index,
	)
	return err
}

const areEVMTxnEventsPersisted = `select 1 from system_evm_events where chain_id=?1 and txn_hash=?2 LIMIT 1`

type AreEVMTxnEventsPersistedParams struct {
	ChainID     uint64
	EventJSON   []byte
	Address     string
	Topics      []byte
	Data        []byte
	BlockNumber uint64
	TxHash      string
	TxIndex     uint
	BlockHash   string
	Index       uint
}

func (q *Queries) AreEVMEventsPersisted(ctx context.Context, arg AreEVMTxnEventsPersistedParams) (bool, error) {
	row := q.queryRow(ctx, q.areEVMEventsPersistedStmt, areEVMTxnEventsPersisted, arg.ChainID, arg.TxHash)
	if row.Err() == sql.ErrNoRows {
		return false, nil
	}
	if row.Err() != nil {
		return false, row.Err()
	}
	return true, nil
}
