-- name: ListPendingTx :many
SELECT * FROM system_pending_tx WHERE address = ?1 AND chain_id = ?2 order by nonce;

-- name: InsertPendingTx :exec
INSERT INTO system_pending_tx (chain_id, address, hash, nonce) VALUES (?1, ?2, ?3, ?4);

-- name: DeletePendingTxByHash :exec
DELETE FROM system_pending_tx WHERE chain_id=?1 AND hash=?2;

-- name: ReplacePendingTxByHash :exec
UPDATE system_pending_tx 
SET hash=?3, bump_price_count=bump_price_count+1, updated_at=?4 
WHERE chain_id=?1 AND hash=?2;