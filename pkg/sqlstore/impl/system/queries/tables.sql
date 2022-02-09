-- name: GetTable :one
SELECT * FROM system_tables WHERE id = $1;

-- name: GetTablesByController :many
SELECT * FROM system_tables WHERE controller LIKE $1;
