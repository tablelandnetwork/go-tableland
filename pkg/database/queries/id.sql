-- name: GetId :one
SELECT id FROM system_id;

-- name: InsertId :exec
INSERT INTO system_id (id) VALUES (?);