package backup

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"sort"
	"strings"

	"github.com/pkg/errors"
)

// Prune prunes the directory keeping the n most recent backup files.
func Prune(dir string, keep int) error {
	if keep < 1 {
		return errors.New("keep less than one")
	}

	files, err := readBackupFiles(dir)
	if err != nil {
		return fmt.Errorf("reading backup files: %s", err)
	}

	if len(files) <= keep {
		return nil
	}

	toBeRemoved := files[:(len(files) - keep)]
	for _, file := range toBeRemoved {
		if err := os.Remove(path.Join(dir, file.Name())); err != nil {
			return errors.Errorf("os remove: %s", err)
		}
	}

	return nil
}

func readBackupFiles(dir string) ([]fs.FileInfo, error) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return []fs.FileInfo{}, fmt.Errorf("read dir: %s", err)
	}

	backupFiles := []fs.FileInfo{}
	for _, f := range files {
		if !strings.HasPrefix(f.Name(), BackupFilenamePrefix) {
			continue
		}

		if !strings.HasSuffix(f.Name(), ".db") && !strings.HasSuffix(f.Name(), ".db.gz") {
			continue
		}

		fi, err := f.Info()
		if err != nil {
			return []fs.FileInfo{}, fmt.Errorf("file info: %s", err)
		}

		backupFiles = append(backupFiles, fi)
	}

	sort.Slice(backupFiles, func(i, j int) bool { return backupFiles[i].ModTime().Before(backupFiles[j].ModTime()) })
	return backupFiles, nil
}
