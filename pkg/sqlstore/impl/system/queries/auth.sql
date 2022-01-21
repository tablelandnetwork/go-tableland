-- name: Authorize :exec
INSERT INTO system_auth ("address") VALUES ($1);

-- name: Revoke :exec
DELETE FROM system_auth WHERE address=$1;

-- name: IsAuthorized :one
SELECT EXISTS(SELECT 1 from system_auth WHERE address=$1) AS "exists";