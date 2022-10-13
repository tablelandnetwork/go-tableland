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

func TestRestorer(t *testing.T) {
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

	dirPath := os.TempDir()
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
