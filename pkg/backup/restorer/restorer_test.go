package restorer

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRestorerWithNoExistingDatabase(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this is a test database that contains the following tables with one record each
		// "a", "system_pending_tx", "system_id"
		f, err := os.Open("testdata/database.db.zst")
		require.NoError(t, err)
		data, err := io.ReadAll(f)
		require.NoError(t, err)
		_, _ = w.Write(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	dirPath := t.TempDir()
	databaseURL := fmt.Sprintf(
		"file://%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL",
		path.Join(dirPath, "database.db"),
	)
	br, err := NewBackupRestorer(ts.URL, databaseURL)
	require.NoError(t, err)
	err = br.Restore()
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", databaseURL)
	require.NoError(t, err)

	var a int
	err = db.QueryRow("SELECT a FROM a LIMIT 1").Scan(&a)
	require.NoError(t, err)
	require.Equal(t, 1, a)

	var c int
	err = db.QueryRow("SELECT count(1) FROM system_pending_tx").Scan(&c)
	require.NoError(t, err)
	require.Equal(t, 0, c)

	err = db.QueryRow("SELECT count(1) FROM system_id").Scan(&c)
	require.NoError(t, err)
	require.Equal(t, 0, c)
}

func TestRestorerWithExistingDatabase(t *testing.T) {
	t.Parallel()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// this is a test database that contains the following tables with one record each
		// "a", "system_pending_tx", "system_id"
		f, err := os.Open("testdata/database.db.zst")
		require.NoError(t, err)
		data, err := io.ReadAll(f)
		require.NoError(t, err)
		_, _ = w.Write(data)
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	dirPath := t.TempDir()

	// we create an existing database with the node id information stored
	createExistingDatabase(t, dirPath)

	databaseURL := fmt.Sprintf(
		"file://%s?_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL",
		path.Join(dirPath, "database.db"),
	)
	br, err := NewBackupRestorer(ts.URL, databaseURL)
	require.NoError(t, err)
	err = br.Restore()
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", databaseURL)
	require.NoError(t, err)

	var a int
	err = db.QueryRow("SELECT a FROM a LIMIT 1").Scan(&a)
	require.NoError(t, err)
	require.Equal(t, 1, a)

	var address string
	err = db.QueryRow("SELECT address FROM system_pending_tx").Scan(&address)
	require.NoError(t, err)
	require.Equal(t, "existing address", address)

	var nodeID string
	err = db.QueryRow("SELECT id FROM system_id").Scan(&nodeID)
	require.NoError(t, err)
	require.Equal(t, "existing node id", nodeID)
}

func createExistingDatabase(t *testing.T, dir string) {
	t.Helper()

	db, err := sql.Open("sqlite3", path.Join(dir, "database.db"))
	require.NoError(t, err)

	_, err = db.Exec("CREATE TABLE IF NOT EXISTS system_id (id TEXT NOT NULL, PRIMARY KEY(id));")
	require.NoError(t, err)

	_, err = db.Exec("INSERT INTO system_id VALUES ('existing node id');")
	require.NoError(t, err)

	_, err = db.Exec(`
		CREATE TABLE system_pending_tx (
			chain_id INTEGER NOT NULL,
			address TEXT NOT NULL,
			hash TEXT NOT NULL,
			nonce INTEGER NOT NULL,
			bump_price_count INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL DEFAULT (strftime('%s', 'now')),
			updated_at INTEGER,
			PRIMARY KEY(chain_id, address, nonce)
		);
	`)
	require.NoError(t, err)

	_, err = db.Exec(`
		INSERT INTO system_pending_tx
		VALUES (1, 'existing address', 'hash', 1, 0, strftime('%s', 'now'), strftime('%s', 'now'));
	`)
	require.NoError(t, err)

	require.NoError(t, db.Close())
}
