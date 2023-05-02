-- name: GetAclByTableAndController :one
SELECT * FROM system_acl WHERE chain_id = ?1 AND table_id = ?2 AND upper(controller) LIKE upper(?3);