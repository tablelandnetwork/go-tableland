package restorer

import (
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/backup"
)

var log = logger.With().Str("component", "backuprestorer").Logger()

// BackupRestorer is responsible for restoring a database from a backup file.
type BackupRestorer struct {
	backupURL string
	dbPath    string
}

// NewBackupRestorer creates a new BackupRestorer.
func NewBackupRestorer(backupURL string, databaseURL string) (*BackupRestorer, error) {
	url, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parsing database url: %s", err)
	}

	return &BackupRestorer{
		backupURL: backupURL,
		dbPath:    url.Path,
	}, nil
}

// Restore restores a database from a backup file URL.
func (br *BackupRestorer) Restore() error {
	defer func() {
		if err := br.cleanUp(); err != nil {
			log.Error().Err(err).Msg("cleaning up")
		}
	}()
	if err := br.downloadBackupFile(br.backupURL, filepath.Dir(br.dbPath)); err != nil {
		return fmt.Errorf("download backup file: %s", err)
	}

	_, err := backup.Decompress(fmt.Sprintf("%s/backup.db.zst", filepath.Dir(br.dbPath)))
	if err != nil {
		return fmt.Errorf("decompress: %s", err)
	}

	if err := br.load(); err != nil {
		return fmt.Errorf("loading the database: %s", err)
	}

	return nil
}

func (br *BackupRestorer) downloadBackupFile(url, dst string) error {
	out, err := os.Create(fmt.Sprintf("%s/backup.db.zst", dst))
	if err != nil {
		return fmt.Errorf("creating backup file: %s", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Error().Err(err).Msg("closing")
		}
	}()

	resp, err := http.Get(url)
	if err != nil {
		return fmt.Errorf("downloading: %s", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			log.Error().Err(err).Msg("closing")
		}
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
	in, err := os.Open(fmt.Sprintf("%s/backup.db", filepath.Dir(br.dbPath)))
	if err != nil {
		return fmt.Errorf("opening file: %s", err)
	}
	defer func() {
		if err := in.Close(); err != nil {
			log.Error().Err(err).Msg("closing")
		}
	}()

	if err := os.Remove(fmt.Sprintf("%s/%s", filepath.Dir(br.dbPath), filepath.Base(br.dbPath))); err != nil {
		log.Warn().Err(err).Msg("removing database file")
	}

	if err := os.Remove(fmt.Sprintf("%s/%s-wal", filepath.Dir(br.dbPath), filepath.Base(br.dbPath))); err != nil {
		log.Warn().Err(err).Msg("removing database wal file")
	}

	if err := os.Remove(fmt.Sprintf("%s/%s-shm", filepath.Dir(br.dbPath), filepath.Base(br.dbPath))); err != nil {
		log.Warn().Err(err).Msg("removing database shm file")
	}

	out, err := os.Create(fmt.Sprintf("%s/%s", filepath.Dir(br.dbPath), filepath.Base(br.dbPath)))
	if err != nil {
		return fmt.Errorf("creating file: %s", err)
	}
	defer func() {
		if err := out.Close(); err != nil {
			log.Error().Err(err).Msg("closing")
		}
	}()

	_, err = io.Copy(out, in)
	if err != nil {
		return fmt.Errorf("copying file: %s", err)
	}

	db, err := sql.Open("sqlite3", fmt.Sprintf("%s/%s", filepath.Dir(br.dbPath), filepath.Base(br.dbPath)))
	if err != nil {
		return fmt.Errorf("opening database: %s", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("closing db")
		}
	}()

	if _, err := db.Exec("DELETE FROM system_pending_tx;"); err != nil {
		return fmt.Errorf("deleting rows from system_pending_tx table: %s", err)
	}

	if _, err := db.Exec("DELETE FROM system_id;"); err != nil {
		return fmt.Errorf("deleting rows from system_id table: %s", err)
	}

	return nil
}

func (br *BackupRestorer) cleanUp() error {
	if err := os.Remove(fmt.Sprintf("%s/backup.db.zst", filepath.Dir(br.dbPath))); err != nil {
		return fmt.Errorf("removing file: %s", err)
	}

	if err := os.Remove(fmt.Sprintf("%s/backup.db", filepath.Dir(br.dbPath))); err != nil {
		return fmt.Errorf("removing file: %s", err)
	}

	return nil
}
