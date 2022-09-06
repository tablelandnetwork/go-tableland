package storage

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" // migration for sqlite3
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"github.com/textileio/go-tableland/pkg/telemetry/storage/migrations"
	"go.opentelemetry.io/otel/attribute"
)

// TelemetryDatabase implements the MetricStore interface and provides storage for a metric.
type TelemetryDatabase struct {
	log   zerolog.Logger
	sqlDB *sql.DB
}

// New returns a new TelemetryDatabase backed by database/sql.
func New(dbURI string) (*TelemetryDatabase, error) {
	sqlDB, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(
		attribute.String("name", "telemetrydb"),
	))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	sqlDB.SetMaxIdleConns(0)
	if err := otelsql.RegisterDBStatsMetrics(sqlDB, otelsql.WithAttributes(
		attribute.String("name", "telemetrydb"),
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}

	log := logger.With().
		Str("component", "telemetrydb").
		Logger()

	db := &TelemetryDatabase{
		log:   log,
		sqlDB: sqlDB,
	}

	as := bindata.Resource(migrations.AssetNames(), migrations.Asset)
	if err := db.executeMigration(dbURI, as); err != nil {
		return nil, fmt.Errorf("initializing db connection: %s", err)
	}

	return db, nil
}

// StoreMetric persists a metric.
func (db *TelemetryDatabase) StoreMetric(ctx context.Context, metric telemetry.Metric) error {
	payloadJSON, err := metric.Serialize()
	if err != nil {
		return fmt.Errorf("marshal json: %s", err)
	}

	_, err = db.sqlDB.ExecContext(ctx,
		`INSERT INTO system_metrics ("timestamp", "type", "payload", "published") VALUES (?1, ?2, ?3, ?4)`,
		metric.Timestamp.UnixMilli(), metric.Type, payloadJSON, 0,
	)
	if err != nil {
		return fmt.Errorf("insert into system_metrics: %s", err)
	}

	return nil
}

// Close closes the database.
func (db *TelemetryDatabase) Close() error {
	if err := db.sqlDB.Close(); err != nil {
		return fmt.Errorf("close: %s", err)
	}

	return nil
}

// executeMigration run db migrations and return a ready to use connection to the SQLite database.
func (db *TelemetryDatabase) executeMigration(dbURI string, as *bindata.AssetSource) error {
	d, err := bindata.WithInstance(as)
	if err != nil {
		return fmt.Errorf("creating source driver: %s", err)
	}

	m, err := migrate.NewWithSourceInstance("go-bindata", d, "sqlite3://"+dbURI)
	if err != nil {
		return fmt.Errorf("creating migration: %s", err)
	}
	version, dirty, err := m.Version()
	db.log.Info().
		Uint("dbVersion", version).
		Bool("dirty", dirty).
		Err(err).
		Msg("database migration executed")

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migration up: %s", err)
	}

	return nil
}
