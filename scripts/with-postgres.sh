#! /bin/sh

echo "Executing postgres initialization..."

quit() {
  echo "$1"
  exit 1
}


[ -z "$POSTGRES_PASSWORD" ] && quit "envvar POSTGRES_PASSWORD is not set"
[ -z "$DB_PASS" ] && quit "envvar DB_PASS is not set. The daemon requires it to connect the database"

POSTGRES_USER="${POSTGRES_USER:-postgres}"
DB_HOST="${DB_HOST:-127.0.0.1:5432}"
DB_NAME="${DB_NAME:-postgres}"
DB_USER=${DB_USER}
# just in case if errant newline sneaked into the k8s secret
POSTGRES_PASSWORD=$(echo -n $POSTGRES_PASSWORD | tr -d \\n)
DB_PASS=$(echo -n $DB_PASS | tr -d \\n)

sleep 5
# when starting the pod, connect to postgres with admin privilege, create user
# and schema if not exist, and force change password, then starts daemon with
# the correct postgres URI. The use is restricted to the schema with the same name.
psql "postgres://$POSTGRES_USER:$POSTGRES_PASSWORD@$DB_HOST" <<EOD
SELECT 'CREATE DATABASE $DB_NAME WITH OWNER $POSTGRES_USER' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$DB_NAME')\gexec
\connect $DB_NAME;

SELECT 'CREATE USER $DB_USER' WHERE NOT EXISTS (SELECT FROM pg_roles WHERE rolname = '$DB_USER')\gexec
GRANT $DB_USER TO $POSTGRES_USER;
ALTER USER $DB_USER WITH PASSWORD '$DB_PASS';
CREATE EXTENSION IF NOT EXISTS pg_stat_statements;
CREATE EXTENSION IF NOT EXISTS uri;
DO \$\$ BEGIN
  CREATE TYPE erc721_metadata AS (
    id int,
    name text,
    description text,
    image uri,
    external_url uri,
    attributes json
  );
EXCEPTION
    WHEN duplicate_object THEN null;
END \$\$;
DO \$\$ BEGIN
  CREATE TYPE erc1155_metadata AS (
    id int,
    name text,
    description text,
    image text, -- Note that we use text here because of the {id} replacement semantics
    external_url uri,
    attributes json
  );
EXCEPTION
    WHEN duplicate_object THEN null;
END \$\$;

EOD
[ $? -eq 0 ] || quit "fail to initialize postgres database"
unset POSTGRES_PASSWORD
unset POSTGRES_USER
$@

echo "Finished"
