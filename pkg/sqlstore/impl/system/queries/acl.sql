-- name: GetAclByTableAndController :one
SELECT * FROM system_acl WHERE chain_id = $3 AND table_id = $2 AND controller ILIKE $1;
