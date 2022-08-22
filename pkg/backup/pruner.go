package backup

import (
	"io/ioutil"
	"os"
	"path"
	"sort"

	"github.com/pkg/errors"
)

// Prune prunes the directory keeping the n most recent files.
func Prune(dir string, keep int) error {
	if keep < 1 {
		return errors.New("keep less than one")
	}

	files, err := ioutil.ReadDir(dir)
	if err != nil {
		return errors.Errorf("read dir: %s", err)
	}

	if len(files) <= keep {
		return nil
	}

	sortFromNewestToOldest(files)

	toBeRemoved := files[keep:]
	for _, file := range toBeRemoved {
		if err := os.Remove(path.Join(dir, file.Name())); err != nil {
			return errors.Errorf("os remove: %s", err)
		}
	}

	return nil
}

func sortFromNewestToOldest(files []os.FileInfo) {
	sort.Slice(files, func(i, j int) bool {
		return files[i].ModTime().Before(files[j].ModTime())
	})
}
