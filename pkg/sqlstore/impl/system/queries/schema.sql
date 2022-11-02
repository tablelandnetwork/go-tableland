-- name: GetSchemaByTableName :one
SELECT sql FROM sqlite_master WHERE name=?1;