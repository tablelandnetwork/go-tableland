package database

import (
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" // migration for sqlite3
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/database/db"
	"github.com/textileio/go-tableland/pkg/database/migrations"
	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
)

// SQLiteDB represents a SQLite database.
type SQLiteDB struct {
	URI     string
	DB      *sql.DB
	Queries *db.Queries
	Log     zerolog.Logger
}

// Open opens a new SQLite database.
func Open(path string, attributes ...attribute.KeyValue) (*SQLiteDB, error) {
	log := logger.With().
		Str("component", "db").
		Logger()

	attributes = append(attributes, metrics.BaseAttrs...)
	sqlDB, err := otelsql.Open("sqlite3", path, otelsql.WithAttributes(attributes...))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}

	if err := otelsql.RegisterDBStatsMetrics(sqlDB, otelsql.WithAttributes(
		attributes...,
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}

	database := &SQLiteDB{
		URI:     path,
		DB:      sqlDB,
		Queries: db.New(sqlDB),
		Log:     log,
	}

	as := bindata.Resource(migrations.AssetNames(), migrations.Asset)
	if err := database.executeMigration(path, as); err != nil {
		return nil, fmt.Errorf("initializing db connection: %s", err)
	}

	return database, nil
}

// Close closes the database.
func (db *SQLiteDB) Close() error {
	return db.DB.Close()
}

// executeMigration run db migrations and return a ready to use connection to the SQLite database.
func (db *SQLiteDB) executeMigration(dbURI string, as *bindata.AssetSource) error {
	d, err := bindata.WithInstance(as)
	if err != nil {
		return fmt.Errorf("creating source driver: %s", err)
	}

	m, err := migrate.NewWithSourceInstance("go-bindata", d, "sqlite3://"+dbURI)
	if err != nil {
		return fmt.Errorf("creating migration: %s", err)
	}
	defer func() {
		if _, err := m.Close(); err != nil {
			db.Log.Error().Err(err).Msg("closing db migration")
		}
	}()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migration up: %s", err)
	}

	version, dirty, err := m.Version()
	db.Log.Info().
		Uint("dbVersion", version).
		Bool("dirty", dirty).
		Err(err).
		Msg("database migration executed")

	return nil
}
