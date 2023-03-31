-- name: GetReceipt :one
SELECT * from system_txn_receipts WHERE chain_id=?1 and txn_hash=?2;