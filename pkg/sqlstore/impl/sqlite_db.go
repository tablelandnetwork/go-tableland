package impl

import (
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" // migration for sqlite3
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/db"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/migrations"
	"go.opentelemetry.io/otel/attribute"
)

// SQLiteDB represents a connection to a SQLite database.
type SQLiteDB struct {
	Log     zerolog.Logger
	DB      *sql.DB
	Queries *db.Queries
}

// NewSQLiteDB returns a new SQLiteDB backed by database/sql.
func NewSQLiteDB(dbURI string) (*SQLiteDB, error) {
	attrs := append([]attribute.KeyValue{
		attribute.String("name", "sqlitedb"),
	},
		metrics.BaseAttrs...)
	dbc, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	if err := otelsql.RegisterDBStatsMetrics(dbc, otelsql.WithAttributes(
		attribute.String("name", "sqlitedb"),
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}

	log := logger.With().
		Str("component", "sqlitedb").
		Logger()

	systemStore := &SQLiteDB{
		Log:     log,
		DB:      dbc,
		Queries: db.New(dbc),
	}

	as := bindata.Resource(migrations.AssetNames(), migrations.Asset)
	if err := systemStore.executeMigration(dbURI, as); err != nil {
		return nil, fmt.Errorf("initializing db connection: %s", err)
	}

	return systemStore, nil
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
	version, dirty, err := m.Version()
	db.Log.Info().
		Uint("dbVersion", version).
		Bool("dirty", dirty).
		Err(err).
		Msg("database migration executed")

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migration up: %s", err)
	}

	return nil
}

// Close closes the database.
func (db *SQLiteDB) Close() error {
	if err := db.DB.Close(); err != nil {
		return fmt.Errorf("closing db: %s", err)
	}
	return nil
}
