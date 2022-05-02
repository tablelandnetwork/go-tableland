-- name: GetTable :one
SELECT * FROM registry WHERE chain_id =$1 AND id = $2;

-- name: GetTablesByController :many
SELECT * FROM registry WHERE chain_id=$1 AND controller ILIKE $2;
