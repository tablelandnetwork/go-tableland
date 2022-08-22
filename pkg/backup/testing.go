package backup

import (
	"io/ioutil"
	"path"
	"testing"

	"github.com/stretchr/testify/require"
)

func createControlDatabase(t *testing.T) DB {
	t.Helper()

	f, err := ioutil.TempFile(t.TempDir(), "control_*.db")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	db, err := open(f.Name())
	require.NoError(t, err)

	query, err := ioutil.ReadFile("testdata/control.sql")
	require.NoError(t, err)

	_, err = db.Exec(string(query))
	require.NoError(t, err)

	return db
}

func backupDir(t *testing.T) string {
	return path.Clean(t.TempDir())
}
