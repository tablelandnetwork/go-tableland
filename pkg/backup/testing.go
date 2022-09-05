package backup

import (
	"os"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func createControlDatabase(t *testing.T) DB {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "control_*.db")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	db, err := open(f.Name())
	require.NoError(t, err)

	query, err := os.ReadFile("testdata/control.sql")
	require.NoError(t, err)

	_, err = db.Exec(string(query))
	require.NoError(t, err)

	return db
}

func backupDir(t *testing.T) string {
	t.Helper()
	return path.Clean(t.TempDir())
}

func requireFileCount(t *testing.T, dir string, counter int) {
	t.Helper()
	files, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, files, counter)
}
