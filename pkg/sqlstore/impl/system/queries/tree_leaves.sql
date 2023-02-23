-- name: FetchChainIDAndBlockNumber :many
SELECT chain_id, block_number FROM system_tree_leaves GROUP BY chain_id, block_number ORDER BY chain_id, block_number;

-- name: FetchLeavesByChainIDAndBlockNumber :many
UPDATE system_tree_leaves SET processing = 1 WHERE chain_id = ?1 AND block_number = ?2 RETURNING *;

-- name: DeleteProcessing :exec
DELETE FROM system_tree_leaves WHERE chain_id = ?1 AND block_number = ?2 AND processing = 1;