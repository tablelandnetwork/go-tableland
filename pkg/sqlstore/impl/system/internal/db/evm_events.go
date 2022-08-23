package db

import (
	"context"
	"database/sql"
	"fmt"
)

const insertEVMEvent = `
INSERT INTO system_evm_events ("chain_id", "event_json", "address", "topics", "data", "block_number", "tx_hash", "tx_index", "block_hash", "event_index")
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10)
`

type InsertEVMEventParams struct {
	ChainID     int64
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

const getEVMEvents = `SELECT * FROM system_evm_events WHERE chain_id=?1 AND tx_hash=?2`

type GetEVMEventsParams struct {
	ChainID int64
	TxHash  string
}

func (q *Queries) GetEVMEvents(ctx context.Context, arg GetEVMEventsParams) ([]EVMEvent, error) {
	rows, err := q.query(ctx, q.getEVMEventsStmt, getEVMEvents, arg.ChainID, arg.TxHash)
	if err != nil {
		return nil, fmt.Errorf("executing getEvmEvents query: %s", err)
	}
	defer rows.Close()

	var ret []EVMEvent
	for rows.Next() {
		var evmEvent EVMEvent
		if err = rows.Scan(
			&evmEvent.ChainID,
			&evmEvent.EventJSON,
			&evmEvent.Address,
			&evmEvent.Topics,
			&evmEvent.Data,
			&evmEvent.BlockNumber,
			&evmEvent.TxHash,
			&evmEvent.TxIndex,
			&evmEvent.BlockHash,
			&evmEvent.Index); err != nil {
			return nil, err
		}
		ret = append(ret, evmEvent)
	}

	return ret, nil
}

const areEVMTxnEventsPersisted = `SELECT 1 FROM system_evm_events where chain_id=?1 and tx_hash=?2 LIMIT 1`

type AreEVMTxnEventsPersistedParams struct {
	ChainID uint64
	TxHash  string
}

func (q *Queries) AreEVMEventsPersisted(ctx context.Context, arg AreEVMTxnEventsPersistedParams) (bool, error) {
	row := q.queryRow(ctx, q.areEVMEventsPersistedStmt, areEVMTxnEventsPersisted, arg.ChainID, arg.TxHash)
	var dummy int
	err := row.Scan(&dummy)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, row.Err()
	}
	return true, nil
}

const getBlocksMissingExtraInfo = `
SELECT DISTINCT e.block_number
FROM system_evm_events e 
LEFT OUTER JOIN system_evm_blocks b ON e.chain_id=b.chain_id AND e.block_number=b.block_number
WHERE e.chain_id=?1 AND b.block_number is null
ORDER BY e.block_number`

type GetBlocksMissingExtraInfoParams struct {
	ChainID int64
}

func (q *Queries) GetBlocksMissingExtraInfo(ctx context.Context, arg GetBlocksMissingExtraInfoParams) ([]int64, error) {
	rows, err := q.query(ctx, q.getBlocksMissingExtraInfoStmt, getBlocksMissingExtraInfo, arg.ChainID)
	if err != nil {
		return nil, fmt.Errorf("executing getBlocksMissingExtraInfo query: %s", err)
	}
	defer rows.Close()

	var ret []int64
	for rows.Next() {
		var blockNumber int64
		if err = rows.Scan(&blockNumber); err != nil {
			return nil, err
		}
		ret = append(ret, blockNumber)
	}

	return ret, nil
}

const insertBlockExtraInfo = `
INSERT INTO system_evm_blocks ("chain_id", "block_number", "timestamp") VALUES (?1, ?2, ?3)`

type InsertBlockExtraInfoParams struct {
	ChainID     int64
	BlockNumber int64
	Timestamp   uint64
}

func (q *Queries) InsertBlockExtraInfo(ctx context.Context, arg InsertBlockExtraInfoParams) error {
	_, err := q.exec(ctx, q.insertBlockExtraInfoStmt, insertBlockExtraInfo,
		arg.ChainID,
		arg.BlockNumber,
		arg.Timestamp,
	)
	return err
}
