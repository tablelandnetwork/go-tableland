-- name: ListPendingTx :many
SELECT * FROM system_pending_tx WHERE address = $1 AND chain_id = $2;

-- name: InsertPendingTx :exec
INSERT INTO system_pending_tx ("chain_id", "address", "hash", "nonce") VALUES ($1, $2, $3, $4);

-- name: DeletePendingTxByHash :exec
DELETE FROM system_pending_tx WHERE hash = $1;