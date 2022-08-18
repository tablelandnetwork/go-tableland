package backup

import (
	"context"
	"database/sql"
	"io/ioutil"
	"os"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestBackuperDefault(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	// substitutes the newBackupFile function to a mocked version
	newBackupFile = func(dir string, _ time.Time) (string, error) {
		timestamp := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
		return createBackupFile(dir, timestamp)
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".")
	require.NoError(t, err)
	require.Equal(t, false, backuper.config.Vacuum)
	require.Equal(t, false, backuper.config.Pruning)
	require.Equal(t, false, backuper.config.Compression)

	result, err := backuper.Backup(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(311296), result.SizeBeforeVacuum)
	require.Equal(t, int64(0), result.SizeAfterVacuum)
	require.Equal(t, time.Duration(0), result.VacuumElapsedTime)
	require.Equal(t, "tbl_backup_2009-11-17T20:34:58Z.db", result.Path)
	require.FileExists(t, "tbl_backup_2009-11-17T20:34:58Z.db")
	require.Greater(t, result.ElapsedTime, time.Duration(0))

	require.NoError(t, backuper.backup.Close())
}

func TestBackuperWithVacuum(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	// substitutes the newBackupFile function to a mocked version
	newBackupFile = func(dir string, _ time.Time) (string, error) {
		timestamp := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
		return createBackupFile(dir, timestamp)
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".", []Option{WithVacuum(true)}...)
	require.NoError(t, err)
	require.Equal(t, true, backuper.config.Vacuum)
	require.Equal(t, false, backuper.config.Pruning)
	require.Equal(t, false, backuper.config.Compression)

	result, err := backuper.Backup(context.Background())

	require.NoError(t, err)
	require.Equal(t, int64(311296), result.SizeBeforeVacuum)
	require.Equal(t, int64(159744), result.SizeAfterVacuum)
	require.Greater(t, result.VacuumElapsedTime, time.Duration(0))
	require.Equal(t, "tbl_backup_2009-11-17T20:34:58Z.db", result.Path)
	require.FileExists(t, "tbl_backup_2009-11-17T20:34:58Z.db")
	require.Greater(t, result.ElapsedTime, time.Duration(0))

	require.NoError(t, backuper.backup.Close())
}

func TestBackuperWithCompression(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	// substitutes the newBackupFile function to a mocked version
	newBackupFile = func(dir string, _ time.Time) (string, error) {
		timestamp := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
		return createBackupFile(dir, timestamp)
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".", []Option{WithVacuum(true), WithCompression(true)}...)
	require.NoError(t, err)
	require.Equal(t, true, backuper.config.Vacuum)
	require.Equal(t, false, backuper.config.Pruning)
	require.Equal(t, true, backuper.config.Compression)

	require.Panicsf(t, func() {
		_, _ = backuper.Backup(context.Background())
	}, "compression not implemented")

	require.NoError(t, backuper.backup.Close())
}

func TestBackuperWithPruning(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	// substitutes the newBackupFile function to a mocked version
	newBackupFile = func(dir string, _ time.Time) (string, error) {
		timestamp := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
		return createBackupFile(dir, timestamp)
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".", []Option{WithVacuum(true), WithPruning(true)}...)
	require.NoError(t, err)
	require.Equal(t, true, backuper.config.Vacuum)
	require.Equal(t, true, backuper.config.Pruning)
	require.Equal(t, false, backuper.config.Compression)

	require.Panicsf(t, func() {
		_, _ = backuper.Backup(context.Background())
	}, "pruning not implemented")

	require.NoError(t, backuper.backup.Close())
}

func TestBackuperMultipleBackupCalls(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".")
	require.NoError(t, err)

	// first call
	_, err = backuper.Backup(context.Background())
	require.NoError(t, err)

	// second call
	result, err := backuper.Backup(context.Background())
	require.NoError(t, err)

	require.NoError(t, err)
	require.Equal(t, int64(311296), result.SizeBeforeVacuum)
	require.Equal(t, int64(0), result.SizeAfterVacuum)
	require.Equal(t, time.Duration(0), result.VacuumElapsedTime)
	require.Greater(t, result.ElapsedTime, time.Duration(0))

	require.NoError(t, backuper.backup.Close())
}

func TestBackuperClose(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".")
	require.NoError(t, err)

	// first call
	_, err = backuper.Backup(context.Background())
	require.NoError(t, err)

	// closes backuper
	require.NoError(t, backuper.Close())

	// second call in a closed backuper throws an error
	_, err = backuper.Backup(context.Background())
	require.ErrorContains(t, err, "database is closed")
}

func TestBackuperBackupError(t *testing.T) {
	// substitutes the open function to a mocked version
	open = func(uri string) (DB, error) {
		db, err := openDatabase(uri)
		require.NoError(t, err)

		return &tempDatabase{db}, nil
	}

	// substitutes the newBackupFile function to a mocked version
	newBackupFile = func(dir string, _ time.Time) (string, error) {
		timestamp := time.Date(2009, 11, 17, 20, 34, 58, 651387237, time.UTC)
		return createBackupFile(dir, timestamp)
	}

	controlDB := CreateControlDatabase(t)
	defer func() {
		require.NoError(t, controlDB.Close())
	}()

	backuper, err := NewBackuper(controlDB.Path(), ".")
	require.NoError(t, err)
	require.FileExists(t, "tbl_backup_2009-11-17T20:34:58Z.db")

	// forcing a DB implementation with broken connection to force an error
	backuper.source = &brokenConnDatabase{backuper.source}

	_, err = backuper.Backup(context.Background())
	require.ErrorContains(t, err, "getting db conn: connection is broken")
	require.NoFileExists(t, "tbl_backup_2009-11-17T20:34:58Z.db") // file was deleted
}

func CreateControlDatabase(t *testing.T) DB {
	db, err := openDatabase("testdata/control.db")
	require.NoError(t, err)

	db = &tempDatabase{db}

	query, err := ioutil.ReadFile("testdata/control.sql")
	require.NoError(t, err)

	_, err = db.Exec(string(query))
	require.NoError(t, err)

	return db
}

// tempDatabase is the a implementation of DB used for testing. It removes the db file when Close is called.
type tempDatabase struct {
	DB
}

func (db *tempDatabase) Close() error {
	defer func() {
		_ = os.Remove(db.Path())
	}()
	return db.DB.Close()
}

type brokenConnDatabase struct {
	DB
}

func (db *brokenConnDatabase) Conn(_ context.Context) (*sql.Conn, error) {
	return nil, errors.New("connection is broken")
}
