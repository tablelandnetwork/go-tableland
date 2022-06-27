package db

import (
	"context"
	"fmt"
)

const getReceipt = `-- name: GetReceipt :one
SELECT chain_id, block_number, block_order, txn_hash, error, table_id from system_txn_receipts WHERE chain_id=?1 and txn_hash=?2
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
		&i.BlockOrder,
		&i.TxnHash,
		&i.Error,
		&i.TableID,
	)
	fmt.Printf("HAHAHAHAHAHHAHA: %#v\n", err)
	return i, err
}
