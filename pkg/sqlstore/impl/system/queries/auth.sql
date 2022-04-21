-- name: IncrementCreateTableCount :exec
UPDATE system_auth SET create_table_count = create_table_count+1, last_seen = NOW() WHERE address ILIKE $1;

-- name: IncrementRunSQLCount :exec
UPDATE system_auth SET run_sql_count = run_sql_count+1, last_seen = NOW() WHERE address ILIKE $1;
