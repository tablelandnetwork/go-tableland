-- name: GetExampleId :one
SELECT id FROM example;

-- name: InsertExample :exec
INSERT INTO example (id) VALUES (?);