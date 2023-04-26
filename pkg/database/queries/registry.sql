-- name: GetTable :one
SELECT * FROM registry WHERE chain_id =?1 AND id = ?2;
