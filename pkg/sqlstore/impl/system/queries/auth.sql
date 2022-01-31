-- name: Authorize :exec
INSERT INTO system_auth ("address") VALUES ($1);

-- name: Revoke :exec
DELETE FROM system_auth WHERE address=$1;

-- name: IsAuthorized :one
SELECT EXISTS(SELECT 1 from system_auth WHERE address=$1) AS "exists";

-- name: GetAuthorized :one
SELECT * FROM system_auth WHERE address=$1;

-- name: ListAuthorized :many
SELECT * FROM system_auth ORDER BY created_at ASC;

-- name: IncrementCreateTableCount :exec
UPDATE system_auth SET create_table_count = create_table_count+1, last_seen = NOW() WHERE address=$1;

-- name: IncrementRunSQLCount :exec
UPDATE system_auth SET run_sql_count = run_sql_count+1, last_seen = NOW() WHERE address=$1;