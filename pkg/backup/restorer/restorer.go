package restorer

import (
	"database/sql"
	"errors"
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

	dbPath := fmt.Sprintf("%s/%s", filepath.Dir(br.dbPath), filepath.Base(br.dbPath))
	previousDBPath := dbPath + ".tmp"

	// If a database already exists we are going to rename it,
	// so, later, we can get the node id information from the existing database.
	dbExists := fileExists(dbPath)
	if dbExists {
		if err := os.Rename(dbPath, previousDBPath); err != nil {
			return fmt.Errorf("renaming file: %s", err)
		}
	}

	if err := os.Remove(fmt.Sprintf("%s/%s-wal", filepath.Dir(br.dbPath), filepath.Base(br.dbPath))); err != nil {
		log.Warn().Err(err).Msg("removing database wal file")
	}

	if err := os.Remove(fmt.Sprintf("%s/%s-shm", filepath.Dir(br.dbPath), filepath.Base(br.dbPath))); err != nil {
		log.Warn().Err(err).Msg("removing database shm file")
	}

	out, err := os.Create(dbPath)
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

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %s", err)
	}
	defer func() {
		if err := db.Close(); err != nil {
			log.Error().Err(err).Msg("closing db")
		}

		if dbExists {
			if err := os.Remove(previousDBPath); err != nil {
				log.Error().Err(err).Msg("removing previous db")
			}
		}
	}()

	// we need to clean up some information not related to the node
	if _, err := db.Exec("DELETE FROM system_pending_tx; DELETE FROM system_id;"); err != nil {
		return fmt.Errorf("deleting rows from system_pending_tx and system_id table: %s", err)
	}

	if dbExists {
		// getting the node id from existing database via attach
		sql := fmt.Sprintf(`
			ATTACH DATABASE '%s' as tmp;
			INSERT INTO system_id SELECT * FROM tmp.system_id;
			INSERT INTO system_pending_tx SELECT * FROM tmp.system_pending_tx;
		`, previousDBPath)
		if _, err := db.Exec(sql); err != nil {
			return fmt.Errorf("copying information from existing database: %s", err)
		}
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

func fileExists(name string) bool {
	_, err := os.Stat(name)
	if err == nil {
		return true
	}
	if errors.Is(err, os.ErrNotExist) {
		return false
	}

	log.Warn().Err(err).Str("file_name", name).Msg("file exists")
	return false
}
