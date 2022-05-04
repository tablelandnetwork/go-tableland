package migrations

import "testing"

func TestMultichainMigration(t *testing.T) {
	// SELECT 'ALTER TABLE ' || table_name || ' RENAME TO _4' || table_name || ';' FROM information_schema.tables where left(table_name, 1) = '_' and left(table_name, 3) <> '_pg' \gexec
}
