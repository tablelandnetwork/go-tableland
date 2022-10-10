package backup

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/mattn/go-sqlite3"

	"github.com/pkg/errors"
)

// BackupFilenamePrefix is the prefix used in every backup file.
const BackupFilenamePrefix = "tbl_backup"

// Backuper is the process that executes the backup process.
type Backuper struct {
	sourcePath, dir string
	source          DB
	backup          DB
	config          *Config

	fileCreator func(string, time.Time) (string, error)
}

// NewBackuper creates a new backuper responsible for making backups of a SQLite database.
func NewBackuper(sourcePath string, backupDir string, opts ...Option) (*Backuper, error) {
	config := DefaultConfig()
	for _, o := range opts {
		if err := o(config); err != nil {
			return nil, err
		}
	}

	if err := os.MkdirAll(backupDir, 0o755); err != nil {
		return nil, errors.Errorf("os mkdir all: %s", err)
	}

	b := &Backuper{
		sourcePath:  sourcePath,
		dir:         backupDir,
		config:      config,
		fileCreator: createBackupFile,
	}

	return b, nil
}

// Backup creates a backup to a file in disk.
// Multiple serial calls to Backup can be perfomed. This can be used to perform retries in case of errors.
func (b *Backuper) Backup(ctx context.Context) (_ BackupResult, err error) {
	defer func() {
		if err != nil {
			_ = os.Remove(b.backup.Path())
		}
	}()

	timestamp, err := b.init()
	if err != nil {
		return BackupResult{}, errors.Errorf("initializing backup: %s", err)
	}

	// SQLite backup starts
	startTime := time.Now()

	connA, err := b.source.Conn(ctx)
	if err != nil {
		return BackupResult{}, errors.Errorf("getting db conn: %s", err)
	}

	connB, err := b.backup.Conn(ctx)
	if err != nil {
		return BackupResult{}, errors.Errorf("getting backup db conn: %s", err)
	}

	if err := b.doBackup(connA, connB); err != nil {
		return BackupResult{}, errors.Errorf("backup: %s", err)
	}

	backupResult := BackupResult{
		Path: b.backup.Path(),
	}
	// SQLite backup finishes
	backupResult.ElapsedTime = time.Since(startTime)

	// gets the size of backup for track record
	backupSize, err := b.getFileSize(b.backup.Path())
	if err != nil {
		return BackupResult{}, errors.Errorf("get file size: %s", err)
	}

	if b.config.Vacuum {
		backupResult.SizeAfterVacuum, backupResult.VacuumElapsedTime, err = b.doVacuum(ctx, connB)
		if err != nil {
			return BackupResult{}, errors.Errorf("do vacuum: %s", err)
		}
	}

	// closes connections because we don't need any more access to dbs
	if err := connA.Close(); err != nil {
		return BackupResult{}, errors.Errorf("closing db connection: %s", err)
	}
	if err := connB.Close(); err != nil {
		return BackupResult{}, errors.Errorf("closing backup connection: %s", err)
	}

	if b.config.Compression {
		backupResult.Path, backupResult.SizeAfterCompression, backupResult.CompressionElapsedTime, err = b.doCompress(b.backup.Path()) // nolint
		if err != nil {
			return BackupResult{}, errors.Errorf("do compress: %s", err)
		}

		if err := os.Remove(b.backup.Path()); err != nil {
			return BackupResult{}, errors.Errorf("os remove: %s", err)
		}
	}

	if b.config.Pruning {
		if err := Prune(b.dir, b.config.KeepFiles); err != nil {
			return BackupResult{}, errors.Errorf("prune: %s", err)
		}
	}

	backupResult.Size = backupSize
	backupResult.Timestamp = timestamp
	return backupResult, nil
}

// Close closes the backuper and backups cannot be taken anymore.
func (b *Backuper) Close() error {
	if err := b.source.Close(); err != nil {
		return errors.Errorf("closing source db: %s", err)
	}
	if err := b.backup.Close(); err != nil {
		return errors.Errorf("closing backup db: %s", err)
	}
	return nil
}

// init opens databases and ping them, then initializes variables.
func (b *Backuper) init() (time.Time, error) {
	source, err := open(b.sourcePath)
	if err != nil {
		return time.Time{}, errors.Errorf("opening source db: %s", err)
	}

	timestamp := time.Now().UTC()
	filename, err := b.fileCreator(b.dir, timestamp)
	if err != nil {
		return time.Time{}, errors.Errorf("creating backup file: %s", err)
	}

	backup, err := open(filename)
	if err != nil {
		return time.Time{}, errors.Errorf("opening backup db: %s", err)
	}

	b.source = source
	b.backup = backup
	return timestamp, nil
}

// doBackup gets raw driver connections and call doBackupRaw.
func (b *Backuper) doBackup(in, out *sql.Conn) error {
	if err := in.Raw(func(driverInConn interface{}) error {
		return out.Raw(func(driverOutConn interface{}) error {
			return b.doBackupRaw(driverInConn.(*sqlite3.SQLiteConn), driverOutConn.(*sqlite3.SQLiteConn))
		})
	}); err != nil {
		return errors.Errorf("backup raw: %s", err)
	}

	return nil
}

// doBackupRaw performs the backup using SQLite backup API.
func (b *Backuper) doBackupRaw(in, out *sqlite3.SQLiteConn) error {
	backup, err := out.Backup("main", in, "main")
	if err != nil {
		return errors.Errorf("failed to initialize the backup: %s", err)
	}

	// copies all pages in one single step
	isDone, err := backup.Step(-1)
	if err != nil {
		return errors.Errorf("failed to perform the backup step: %s", err)
	}
	if !isDone {
		return errors.New("backup is unexpectedly not done")
	}

	// double-check to make sure there's no more page remaining
	if finalRemaining := backup.Remaining(); finalRemaining != 0 {
		return errors.Errorf("unexpected remaining value: %d", finalRemaining)
	}

	if err := backup.Finish(); err != nil {
		return errors.Errorf("failed to finish backup: %s", err)
	}

	return nil
}

func (b *Backuper) doVacuum(ctx context.Context, conn *sql.Conn) (int64, time.Duration, error) {
	startTime := time.Now()
	if _, err := conn.ExecContext(ctx, "VACUUM"); err != nil {
		return 0, 0, errors.Errorf("exec vacuum: %s", err)
	}

	size, err := b.getFileSize(b.backup.Path())
	if err != nil {
		return 0, 0, errors.Errorf("get file size: %s", err)
	}

	return size, time.Since(startTime), nil
}

func (b *Backuper) doCompress(filepath string) (string, int64, time.Duration, error) {
	startTime := time.Now()
	newFilepath, err := Compress(filepath)
	if err != nil {
		return "", 0, 0, errors.Errorf("compress: %s", err)
	}

	size, err := b.getFileSize(newFilepath)
	if err != nil {
		return "", 0, 0, errors.Errorf("get file size: %s", err)
	}

	return newFilepath, size, time.Since(startTime), nil
}

func (b *Backuper) getFileSize(filename string) (int64, error) {
	fi, err := os.Stat(filename)
	if err != nil {
		return 0, errors.Errorf("os stat: %s", err)
	}

	return fi.Size(), nil
}

func open(uri string) (DB, error) {
	db, err := sql.Open("sqlite3", uri)
	if err != nil {
		return nil, errors.Errorf("opening db: %s", err)
	}
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(1)

	if err := db.Ping(); err != nil {
		return nil, errors.Errorf("pinging db: %s", err)
	}

	return &Database{uri, db}, nil
}

func createBackupFile(dir string, timestamp time.Time) (string, error) {
	filename := path.Join(dir, fmt.Sprintf("%s_%s.db", BackupFilenamePrefix, timestamp.Format(time.RFC3339)))
	backupFile, err := os.Create(filename)
	if err != nil {
		return "", errors.Errorf("os create: %s", err)
	}
	if err := backupFile.Close(); err != nil {
		return "", errors.Errorf("closing backup file: %s", err)
	}
	return filename, nil
}

// BackupResult represents the result of a backup process.
type BackupResult struct {
	Timestamp time.Time
	Path      string

	// Stats
	ElapsedTime            time.Duration
	VacuumElapsedTime      time.Duration
	CompressionElapsedTime time.Duration
	Size                   int64
	SizeAfterVacuum        int64
	SizeAfterCompression   int64
}

// DB is a subset of *sql.DB operations used in Backuper. This interfaces aids with testing.
type DB interface {
	Close() error
	Ping() error
	SetMaxOpenConns(n int)
	Conn(context.Context) (*sql.Conn, error)
	Exec(query string, args ...interface{}) (sql.Result, error)

	// new
	Path() string
}

// Database is the implementation of DB used in Backuper. It inherits *sql.DB.
type Database struct {
	path string
	*sql.DB
}

// Path returns the path of the database.
func (db *Database) Path() string {
	return db.path
}

// Config contains configuration parameters for backuper.
type Config struct {
	Compression bool
	Pruning     bool
	Vacuum      bool
	KeepFiles   int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Compression: false,
		Pruning:     false,
		Vacuum:      false,
		KeepFiles:   5,
	}
}

// Option modifies a configuration attribute.
type Option func(*Config) error

// WithCompression enables compression.
func WithCompression(v bool) Option {
	return func(c *Config) error {
		c.Compression = v
		return nil
	}
}

// WithPruning enables pruning of old backup files.
func WithPruning(v bool, keep int) Option {
	return func(c *Config) error {
		c.Pruning = v
		c.KeepFiles = keep
		return nil
	}
}

// WithVacuum enables VACUUM operation.
func WithVacuum(v bool) Option {
	return func(c *Config) error {
		c.Vacuum = v
		return nil
	}
}
