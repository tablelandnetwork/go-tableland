-- name: GetAclByTableAndController :one
SELECT * FROM system_acl WHERE table_id = $2 and controller ILIKE $1;
