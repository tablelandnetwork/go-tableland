-- name: GetNonce :one
SELECT * FROM system_nonce WHERE address = $1 AND network = $2;

-- name: UpsertNonce :exec
INSERT INTO system_nonce ("network", "address", "nonce") VALUES ($1, $2, $3)
    ON CONFLICT (network, address)
    DO UPDATE SET nonce = $3
    WHERE system_nonce.address = $2 AND system_nonce.network = $1;

-- name: ListPendingTx :many
SELECT * FROM system_pending_tx WHERE address = $1 AND network = $2;

-- name: InsertPendingTx :exec
INSERT INTO system_pending_tx ("network", "address", "hash", "nonce") VALUES ($1, $2, $3, $4);

-- name: DeletePendingTxByHash :exec
DELETE FROM system_pending_tx WHERE hash = $1;