package system

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog/log"

	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // triggers something?
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/pkg/parsing"
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

// GetTable fetchs a table from its UUID.
func (s *SystemStore) GetTable(ctx context.Context, id parsing.TableID) (sqlstore.Table, error) {
	dbID := pgtype.Numeric{}
	dbID.Set(id.ToBigInt())
	table, err := s.db.GetTable(ctx, dbID)
	if err != nil {
		return sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}

	return tableFromSQLToDTO(table), nil
}

// GetTablesByController fetchs a table from controller address.
func (s *SystemStore) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	sqlcTables, err := s.db.GetTablesByController(ctx, controller)
	if err != nil {
		return []sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}

	tables := make([]sqlstore.Table, len(sqlcTables))
	for i := range sqlcTables {
		tables[i] = tableFromSQLToDTO(sqlcTables[i])
	}

	return tables, nil
}

// Authorize grants the provided address permission to use the system.
func (s *SystemStore) Authorize(ctx context.Context, address string) error {
	if err := s.db.Authorize(ctx, address); err != nil {
		return fmt.Errorf("authorizating: %s", err)
	}
	return nil
}

// Revoke removes permission to use the system from the provided address.
func (s *SystemStore) Revoke(ctx context.Context, address string) error {
	if err := s.db.Revoke(ctx, address); err != nil {
		return fmt.Errorf("revoking: %s", err)
	}
	return nil
}

// IsAuthorized checks if the provided address has permission to use the system.
func (s *SystemStore) IsAuthorized(ctx context.Context, address string) (sqlstore.IsAuthorizedResult, error) {
	authorized, err := s.db.IsAuthorized(ctx, address)
	if err != nil {
		return sqlstore.IsAuthorizedResult{}, fmt.Errorf("checking authorization: %s", err)
	}
	return sqlstore.IsAuthorizedResult{IsAuthorized: authorized}, nil
}

// GetAuthorizationRecord gets the authorization record for the provided address.
func (s *SystemStore) GetAuthorizationRecord(
	ctx context.Context,
	address string,
) (sqlstore.AuthorizationRecord, error) {
	res, err := s.db.GetAuthorized(ctx, address)
	if err != nil {
		return sqlstore.AuthorizationRecord{}, fmt.Errorf("getthing authorization record: %s", err)
	}
	if res.Address == "" {
		return sqlstore.AuthorizationRecord{}, fmt.Errorf("address not authorized")
	}
	var lastSeen *time.Time
	if res.LastSeen.Valid {
		lastSeen = &res.LastSeen.Time
	}
	return sqlstore.AuthorizationRecord{
		Address:          res.Address,
		CreatedAt:        res.CreatedAt,
		LastSeen:         lastSeen,
		CreateTableCount: res.CreateTableCount,
		RunSQLCount:      res.RunSqlCount,
	}, nil
}

// ListAuthorized returns a list of all authorization records.
func (s *SystemStore) ListAuthorized(ctx context.Context) ([]sqlstore.AuthorizationRecord, error) {
	res, err := s.db.ListAuthorized(ctx)
	if err != nil {
		return nil, fmt.Errorf("getthing authorization records: %s", err)
	}
	records := make([]sqlstore.AuthorizationRecord, 0)
	for _, r := range res {
		var lastSeen *time.Time
		if r.LastSeen.Valid {
			lastSeen = &r.LastSeen.Time
		}
		records = append(records,
			sqlstore.AuthorizationRecord{
				Address:          r.Address,
				CreatedAt:        r.CreatedAt,
				LastSeen:         lastSeen,
				CreateTableCount: r.CreateTableCount,
				RunSQLCount:      r.RunSqlCount,
			},
		)
	}
	return records, nil
}

// IncrementCreateTableCount increments the counter.
func (s *SystemStore) IncrementCreateTableCount(ctx context.Context, address string) error {
	if err := s.db.IncrementCreateTableCount(ctx, address); err != nil {
		return fmt.Errorf("incrementing create table count: %s", err)
	}
	return nil
}

// IncrementRunSQLCount increments the counter.
func (s *SystemStore) IncrementRunSQLCount(ctx context.Context, address string) error {
	if err := s.db.IncrementRunSQLCount(ctx, address); err != nil {
		return fmt.Errorf("incrementing run sql count: %s", err)
	}
	return nil
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
	log.Info().
		Uint("dbVersion", version).
		Bool("dirty", dirty).
		Err(err).
		Msg("database migration executed")

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("running migration up: %s", err)
	}

	return nil
}

func tableFromSQLToDTO(table db.SystemTable) sqlstore.Table {
	return sqlstore.Table{
		ID:          parsing.TableID(*table.ID.Int),
		Controller:  table.Controller,
		Name:        table.Name,
		Description: table.Description,
		Structure:   table.Structure,
		CreatedAt:   table.CreatedAt,
	}
}
