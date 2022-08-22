package backup

import (
	"io/ioutil"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPruner(t *testing.T) {
	t.Parallel()

	t.Run("number of files less than keep", func(t *testing.T) {
		t.Parallel()
		testPruner(t, 2, 4)
	})

	t.Run("number of files equals than keep", func(t *testing.T) {
		t.Parallel()
		testPruner(t, 2, 2)
	})

	t.Run("number of files greater than keep", func(t *testing.T) {
		t.Parallel()
		testPruner(t, 4, 2)
	})
}

func testPruner(t *testing.T, n, keep int) {
	t.Helper()
	dir := t.TempDir()
	modTime := make([]time.Time, n)
	for i := 0; i < n; i++ {
		f, err := ioutil.TempFile(dir, "")
		require.NoError(t, err)

		fi, err := f.Stat()
		require.NoError(t, err)
		modTime[i] = fi.ModTime()

		// wait a bit to make sure next file has a mod time different than previous files
		time.Sleep(100 * time.Millisecond)
	}
	requireFileCount(t, dir, n)
	require.IsIncreasing(t, modTime)

	err := Prune(dir, keep)
	require.NoError(t, err)

	requireFileCount(t, dir, min(n, keep))

	files, err := ioutil.ReadDir(dir)
	require.NoError(t, err)

	sortFromNewestToOldest(files)

	expModTime := make([]time.Time, min(n, keep))
	for i := 0; i < min(n, keep); i++ {
		expModTime[i] = modTime[i]
	}

	gotModTime := make([]time.Time, min(n, keep))
	for i := 0; i < min(n, keep); i++ {
		gotModTime[i] = files[i].ModTime()
	}
	require.ElementsMatch(t, expModTime, gotModTime)
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}
