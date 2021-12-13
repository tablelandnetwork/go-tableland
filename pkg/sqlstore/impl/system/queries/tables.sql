-- name: InsertTable :exec
INSERT INTO system_tables (
    uuid,
    controller
    ) VALUES (
      $1,
      $2);

-- name: GetTable :one
SELECT * FROM system_tables WHERE uuid = $1;