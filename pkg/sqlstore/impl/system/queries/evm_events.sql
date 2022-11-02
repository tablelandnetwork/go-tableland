-- name: InsertEVMEvent :exec
INSERT INTO system_evm_events (chain_id, event_json, event_type, address, topics, data, block_number, tx_hash, tx_index, block_hash, event_index)
VALUES (?1, ?2, ?3, ?4, ?5, ?6, ?7, ?8, ?9, ?10, ?11);

-- name: GetEVMEvents :many
SELECT * FROM system_evm_events WHERE chain_id=?1 AND tx_hash=?2;

-- name: AreEVMEventsPersisted :one
SELECT 1 FROM system_evm_events where chain_id=?1 and tx_hash=?2 LIMIT 1;

-- name: GetBlocksMissingExtraInfoByBlockNumber :many
SELECT DISTINCT e.block_number
FROM system_evm_events e 
WHERE e.chain_id=?1 AND e.block_number>?2 AND
NOT EXISTS(select* from system_evm_blocks b WHERE e.chain_id=b.chain_id AND e.block_number=b.block_number)
ORDER BY e.block_number;

-- name: GetBlocksMissingExtraInfo :many
SELECT DISTINCT e.block_number
FROM system_evm_events e 
WHERE e.chain_id=?1 AND NOT EXISTS(select * from system_evm_blocks b WHERE e.chain_id=b.chain_id AND e.block_number=b.block_number)
ORDER BY e.block_number;

-- name: GetBlockExtraInfo :one
SELECT * FROM system_evm_blocks WHERE chain_id=?1 and block_number=?2;

-- name: InsertBlockExtraInfo :exec
INSERT INTO system_evm_blocks (chain_id, block_number, timestamp) VALUES (?1, ?2, ?3);