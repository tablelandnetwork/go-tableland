package restorer

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"

	"github.com/textileio/go-tableland/pkg/backup"
)

// BackupRestorer is responsible for restoring a database from a backup file.
type BackupRestorer struct {
	url, dst string
}

// NewBackupRestorer creates a new BackupRestorer.
func NewBackupRestorer(url string, dst string) *BackupRestorer {
	return &BackupRestorer{
		url: url,
		dst: dst,
	}
}

// Restore restores a database from a backup file URL.
func (br *BackupRestorer) Restore() error {
	if err := br.downloadBackupFile(br.url, br.dst); err != nil {
		return fmt.Errorf("download backup file: %s", err)
	}

	_, err := backup.Decompress(fmt.Sprintf("%s/backup.db.zst", br.dst))
	if err != nil {
		return fmt.Errorf("decompress: %s", err)
	}

	if err := br.load(); err != nil {
		return fmt.Errorf("loading the database: %s", err)
	}

	if err := br.cleanUp(); err != nil {
		return fmt.Errorf("cleaning up: %s", err)
	}

	return nil
}

func (br *BackupRestorer) downloadBackupFile(url, dst string) error {
	out, err := os.Create(fmt.Sprintf("%s/backup.db.zst", dst))
	if err != nil {
		return fmt.Errorf("creating backup file: %s", err)
	}
	defer func() {
		_ = out.Close()
	}()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading: %s", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status: %s", resp.Status)
	}

	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return fmt.Errorf("io copy: %s", err)
	}

	return nil
}

func (br *BackupRestorer) load() error {
	in, err := os.Open(fmt.Sprintf("%s/backup.db", br.dst))
	if err != nil {
		return fmt.Errorf("opening file: %s", err)
	}
	defer func() {
		_ = in.Close()
	}()

	out, err := os.Create(fmt.Sprintf("%s/database.db", br.dst))
	if err != nil {
		return fmt.Errorf("creating file: %s", err)
	}
	defer func() {
		_ = out.Close()
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("copying file: %s", err)
	}
	return nil
}

func (br *BackupRestorer) cleanUp() error {
	db, err := sql.Open("sqlite3", fmt.Sprintf("%s/database.db", br.dst))
	if err != nil {
		return fmt.Errorf("removing file: %s", err)
	}

	if _, err := db.Exec("DELETE FROM system_pending_tx;"); err != nil {
		return fmt.Errorf("deleting rows from system_pending_tx file: %s", err)
	}

	if err := os.Remove(fmt.Sprintf("%s/backup.db.zst", br.dst)); err != nil {
		return fmt.Errorf("removing file: %s", err)
	}

	if err := os.Remove(fmt.Sprintf("%s/backup.db", br.dst)); err != nil {
		return fmt.Errorf("removing file: %s", err)
	}

	return nil
}
