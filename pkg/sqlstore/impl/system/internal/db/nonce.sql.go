package db

import (
	"context"
	"time"
)

const deletePendingTxByHash = `
DELETE FROM system_pending_tx WHERE chain_id=?1 AND hash=?2
`

type DeletePendingTxByHashParams struct {
	ChainID int64
	Hash    string
}

func (q *Queries) DeletePendingTxByHash(ctx context.Context, arg DeletePendingTxByHashParams) error {
	_, err := q.exec(ctx, q.deletePendingTxByHashStmt, deletePendingTxByHash, arg.ChainID, arg.Hash)
	return err
}

const insertPendingTx = `
INSERT INTO system_pending_tx ("chain_id", "address", "hash", "nonce", "created_at") 
VALUES (?1, ?2, ?3, ?4, ?5)
`

type InsertPendingTxParams struct {
	ChainID int64
	Address string
	Hash    string
	Nonce   int64
}

func (q *Queries) InsertPendingTx(ctx context.Context, arg InsertPendingTxParams) error {
	_, err := q.exec(ctx, q.insertPendingTxStmt, insertPendingTx,
		arg.ChainID,
		arg.Address,
		arg.Hash,
		arg.Nonce,
		time.Now().Unix(),
	)
	return err
}

const listPendingTx = `
SELECT chain_id, address, hash, nonce, created_at, bump_price_count FROM system_pending_tx WHERE address = ?1 AND chain_id = ?2 order by nonce
`

type ListPendingTxParams struct {
	Address string
	ChainID int64
}

func (q *Queries) ListPendingTx(ctx context.Context, arg ListPendingTxParams) ([]SystemPendingTx, error) {
	rows, err := q.query(ctx, q.listPendingTxStmt, listPendingTx, arg.Address, arg.ChainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []SystemPendingTx
	for rows.Next() {
		var i SystemPendingTx
		var createdAtUnix int64
		if err := rows.Scan(
			&i.ChainID,
			&i.Address,
			&i.Hash,
			&i.Nonce,
			&createdAtUnix,
			&i.BumpPriceCount,
		); err != nil {
			return nil, err
		}
		i.CreatedAt = time.Unix(createdAtUnix, 0)
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const replacePendingTxByHash = `
UPDATE system_pending_tx 
SET hash=?3, bump_price_count=bump_price_count+1, updated_at=?4
WHERE chain_id=?1 AND hash=?2
`

type ReplacePendingTxByHashParams struct {
	ChainID      int64
	PreviousHash string
	NewHash      string
}

func (q *Queries) ReplacePendingTxByHash(ctx context.Context, arg ReplacePendingTxByHashParams) error {
	_, err := q.db.ExecContext(ctx, replacePendingTxByHash, arg.ChainID, arg.PreviousHash, arg.NewHash, time.Now().Unix())
	return err
}
