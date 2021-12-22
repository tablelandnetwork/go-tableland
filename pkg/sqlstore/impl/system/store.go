package system

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // triggers something?
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/internal/db"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/migrations"
)

// SystemStore provides a persistent layer for storage requests.
type SystemStore struct {
	db *db.Queries
}

// New returns a new SystemStore backed by `pgxpool.Pool`.
func New(pool *pgxpool.Pool) (*SystemStore, error) {
	as := bindata.Resource(migrations.AssetNames(),
		func(name string) ([]byte, error) {
			return migrations.Asset(name)
		})
	err := executeMigration(pool.Config().ConnString(), as)
	if err != nil {
		return nil, fmt.Errorf("initializing db connection: %s", err)
	}

	return &SystemStore{db: db.New(pool)}, nil
}

// InsertTable inserts a new system-wide table.
func (s *SystemStore) InsertTable(ctx context.Context, uuid uuid.UUID, controller string) error {
	err := s.db.InsertTable(ctx, db.InsertTableParams{
		UUID:       uuid,
		Controller: controller,
	})

	if err != nil {
		return fmt.Errorf("failed to insert a new table: %s", err)
	}

	return nil
}

// GetTable fetchs a table from its UUID.
func (s *SystemStore) GetTable(ctx context.Context, uuid uuid.UUID) (sqlstore.Table, error) {
	table, err := s.db.GetTable(ctx, uuid)
	if err != nil {
		return sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}
	return sqlstore.Table{UUID: table.UUID, Controller: table.Controller, CreatedAt: table.CreatedAt}, err
}

// GetTablesByController fetchs a table from controller address.
func (s *SystemStore) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	sqlcTables, err := s.db.GetTablesByController(ctx, controller)
	if err != nil {
		return []sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}

	tables := make([]sqlstore.Table, 0)
	for _, t := range sqlcTables {
		tables = append(tables, sqlstore.Table{UUID: t.UUID, Controller: t.Controller, CreatedAt: t.CreatedAt})
	}
	return tables, err
}

// executeMigration run db migrations and return a ready to use connection to the Postgres database.
func executeMigration(postgresURI string, as *bindata.AssetSource) error {
	// To avoid dealing with time zone issues, we just enforce UTC timezone
	if !strings.Contains(postgresURI, "timezone=UTC") {
		return errors.New("timezone=UTC is required in postgres URI")
	}
	d, err := bindata.WithInstance(as)
	if err != nil {
		return fmt.Errorf("creating source driver: %s", err)
	}

	m, err := migrate.NewWithSourceInstance("go-bindata", d, postgresURI)
	if err != nil {
		return fmt.Errorf("creating migration: %s", err)
	}
	version, dirty, err := m.Version()
	log.Printf("current version %d, dirty %v, err: %v", version, dirty, err)
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migration up: %s", err)
	}

	return nil
}
