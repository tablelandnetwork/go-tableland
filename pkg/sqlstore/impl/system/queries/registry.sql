-- name: GetTable :one
SELECT * FROM registry WHERE chain_id =?1 AND id = ?2;

-- name: GetTablesByController :many
SELECT * FROM registry WHERE chain_id=?1 AND upper(controller) LIKE upper(?2);

-- name: GetTablesByStructure :many
SELECT * FROM registry WHERE chain_id=?1 AND structure=?2;