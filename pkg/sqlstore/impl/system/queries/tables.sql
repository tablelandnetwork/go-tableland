-- name: GetTable :one
SELECT * FROM system_tables WHERE uuid = $1;

-- name: GetTablesByController :many
SELECT * FROM system_tables WHERE controller = $1;
