package backup

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestPruner(t *testing.T) {
	t.Parallel()

	for n := 1; n <= 10; n++ {
		for keep := 1; keep <= 5; keep++ {
			t.Run(fmt.Sprintf("%d-%d", n, keep), func(t *testing.T) {
				t.Parallel()
				testPruner(t, n, keep)
			})
		}
	}
}

func testPruner(t *testing.T, n, keep int) {
	t.Helper()
	dir := t.TempDir()
	modTime := make([]int64, n)
	for i := 0; i < n; i++ {
		f, err := ioutil.TempFile(dir, fmt.Sprintf("%s*.db", BackupFilenamePrefix))
		require.NoError(t, err)

		fi, err := f.Stat()
		require.NoError(t, err)
		modTime[i] = fi.ModTime().UnixNano()

		// wait a bit to make sure next file has a mod time different than previous files
		time.Sleep(100 * time.Millisecond)
	}
	requireFileCount(t, dir, n)
	require.IsIncreasing(t, modTime)

	err := Prune(dir, keep)
	require.NoError(t, err)

	requireFileCount(t, dir, min(n, keep))

	files, err := os.ReadDir(dir)
	require.NoError(t, err)

	expModTime := make([]int64, min(n, keep))
	for i := 0; i < min(n, keep); i++ {
		if keep < n {
			expModTime[i] = modTime[i+(len(modTime)-keep)]
		} else {
			expModTime[i] = modTime[i]
		}
	}
	gotModTime := make([]int64, min(n, keep))
	for i := min(n, keep) - 1; i >= 0; i-- {
		fi, err := files[i].Info()
		require.NoError(t, err)
		gotModTime[i] = fi.ModTime().UnixNano()
	}
	require.ElementsMatch(t, expModTime, gotModTime)
}

func min(a, b int) int {
	if a < b {
		return a
	}

	return b
}
