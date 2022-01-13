-- name: InsertTable :exec
INSERT INTO system_tables (
    "uuid",
    "controller",
    "type"
    ) VALUES (
      $1,
      $2,
      $3);

-- name: GetTable :one
SELECT * FROM system_tables WHERE uuid = $1;

-- name: GetTablesByController :many
SELECT * FROM system_tables WHERE controller = $1;