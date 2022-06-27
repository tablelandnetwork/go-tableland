package db

import (
	"context"
)

const getReceipt = `-- name: GetReceipt :one
SELECT chain_id, block_number, index_in_block, txn_hash, error, table_id from system_txn_receipts WHERE chain_id=?1 and txn_hash=?2
`

type GetReceiptParams struct {
	ChainID int64
	TxnHash string
}

func (q *Queries) GetReceipt(ctx context.Context, arg GetReceiptParams) (SystemTxnReceipt, error) {
	row := q.queryRow(ctx, q.getReceiptStmt, getReceipt, arg.ChainID, arg.TxnHash)
	var i SystemTxnReceipt
	err := row.Scan(
		&i.ChainID,
		&i.BlockNumber,
		&i.IndexInBlock,
		&i.TxnHash,
		&i.Error,
		&i.TableID,
	)
	return i, err
}
