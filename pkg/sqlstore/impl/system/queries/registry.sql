-- name: GetTable :one
SELECT * FROM registry WHERE id = $1;

-- name: GetTablesByController :many
SELECT * FROM registry WHERE controller ILIKE $1;
