-- name: InsertTable :exec
INSERT INTO tables (
    uuid,
    controller
    ) VALUES (
      $1,
      $2);

-- name: GetTable :one
SELECT * FROM tables WHERE uuid = $1;