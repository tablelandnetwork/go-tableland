package system

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"

	"strings"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres" // triggers something?
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/internal/db"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/migrations"
)

// SystemStore provides a persistent layer for storage requests.
// The methods implemented by this layer can be executed inside a given transaction or not.
// For safety reasons, this layer has no access to the database object or the transaction object.
// The access is made through the dbWithTx interface.
type SystemStore struct {
	db dbWithTx
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

	return &SystemStore{db: &dbWithTxImpl{db: db.New(pool)}}, nil
}

// GetTable fetchs a table from its UUID.
func (s *SystemStore) GetTable(ctx context.Context, id tableland.TableID) (sqlstore.Table, error) {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(id.String()); err != nil {
		return sqlstore.Table{}, fmt.Errorf("parsing id to numeric: %s", err)
	}
	table, err := s.db.queries().GetTable(ctx, dbID)
	if err != nil {
		return sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}
	return tableFromSQLToDTO(table)
}

// GetTablesByController fetchs a table from controller address.
func (s *SystemStore) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	if err := sanitizeAddress(controller); err != nil {
		return []sqlstore.Table{}, fmt.Errorf("sanitizing address: %s", err)
	}
	sqlcTables, err := s.db.queries().GetTablesByController(ctx, controller)
	if err != nil {
		return []sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}

	tables := make([]sqlstore.Table, len(sqlcTables))
	for i := range sqlcTables {
		tables[i], err = tableFromSQLToDTO(sqlcTables[i])
		if err != nil {
			return nil, fmt.Errorf("parsing database table to dto: %s", err)
		}
	}

	return tables, nil
}

// Authorize grants the provided address permission to use the system.
func (s *SystemStore) Authorize(ctx context.Context, address string) error {
	if err := sanitizeAddress(address); err != nil {
		return fmt.Errorf("sanitizing address: %s", err)
	}
	if err := s.db.queries().Authorize(ctx, address); err != nil {
		return fmt.Errorf("authorizating: %s", err)
	}
	return nil
}

// Revoke removes permission to use the system from the provided address.
func (s *SystemStore) Revoke(ctx context.Context, address string) error {
	if err := sanitizeAddress(address); err != nil {
		return fmt.Errorf("sanitizing address: %s", err)
	}
	if err := s.db.queries().Revoke(ctx, address); err != nil {
		return fmt.Errorf("revoking: %s", err)
	}
	return nil
}

// IsAuthorized checks if the provided address has permission to use the system.
func (s *SystemStore) IsAuthorized(ctx context.Context, address string) (sqlstore.IsAuthorizedResult, error) {
	if err := sanitizeAddress(address); err != nil {
		return sqlstore.IsAuthorizedResult{}, fmt.Errorf("sanitizing address: %s", err)
	}
	authorized, err := s.db.queries().IsAuthorized(ctx, address)
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
	if err := sanitizeAddress(address); err != nil {
		return sqlstore.AuthorizationRecord{}, fmt.Errorf("sanitizing address: %s", err)
	}
	res, err := s.db.queries().GetAuthorized(ctx, address)
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
	res, err := s.db.queries().ListAuthorized(ctx)
	if err != nil {
		return nil, fmt.Errorf("getthing authorization records: %s", err)
	}

	records := make([]sqlstore.AuthorizationRecord, 0)
	for _, r := range res {
		rec := sqlstore.AuthorizationRecord{
			Address:          r.Address,
			CreatedAt:        r.CreatedAt,
			CreateTableCount: r.CreateTableCount,
			RunSQLCount:      r.RunSqlCount,
		}
		if r.LastSeen.Valid {
			lastSeen := r.LastSeen.Time
			rec.LastSeen = &lastSeen
		}
		records = append(records, rec)
	}

	return records, nil
}

// IncrementCreateTableCount increments the counter.
func (s *SystemStore) IncrementCreateTableCount(ctx context.Context, address string) error {
	if err := sanitizeAddress(address); err != nil {
		return fmt.Errorf("sanitizing address: %s", err)
	}
	if err := s.db.queries().IncrementCreateTableCount(ctx, address); err != nil {
		return fmt.Errorf("incrementing create table count: %s", err)
	}
	return nil
}

// IncrementRunSQLCount increments the counter.
func (s *SystemStore) IncrementRunSQLCount(ctx context.Context, address string) error {
	if err := sanitizeAddress(address); err != nil {
		return fmt.Errorf("sanitizing address: %s", err)
	}
	if err := s.db.queries().IncrementRunSQLCount(ctx, address); err != nil {
		return fmt.Errorf("incrementing run sql count: %s", err)
	}
	return nil
}

// GetACLOnTableByController returns the privileges on table stored in the database for a given controller.
func (s *SystemStore) GetACLOnTableByController(
	ctx context.Context,
	id tableland.TableID,
	controller string) (sqlstore.SystemACL, error) {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(id.String()); err != nil {
		return sqlstore.SystemACL{}, fmt.Errorf("parsing table id to numeric: %s", err)
	}

	params := db.GetAclByTableAndControllerParams{
		Controller: controller,
		TableID:    dbID,
	}

	systemACL, err := s.db.queries().GetAclByTableAndController(ctx, params)
	if err == pgx.ErrNoRows {
		return sqlstore.SystemACL{
			Controller: controller,
			TableID:    id,
		}, nil
	}

	if err != nil {
		return sqlstore.SystemACL{}, fmt.Errorf("failed to get the acl info: %s", err)
	}

	return aclFromSQLtoDTO(systemACL)
}

// GetNonce returns the nonce stored in the database by a given address.
func (s *SystemStore) GetNonce(ctx context.Context, network string, addr common.Address) (sqlstore.Nonce, error) {
	params := db.GetNonceParams{
		Address: addr.Hex(),
		Network: network,
	}

	systemNonce, err := s.db.queries().GetNonce(ctx, params)
	if err == pgx.ErrNoRows {
		return sqlstore.Nonce{
			Address: common.HexToAddress(systemNonce.Address),
			Network: systemNonce.Network,
		}, nil
	}

	if err != nil {
		return sqlstore.Nonce{}, fmt.Errorf("get nonce: %s", err)
	}

	return sqlstore.Nonce{
		Address: common.HexToAddress(systemNonce.Address),
		Network: systemNonce.Network,
		Nonce:   systemNonce.Nonce,
	}, nil
}

// UpsertNonce updates a nonce.
func (s *SystemStore) UpsertNonce(ctx context.Context, network string, addr common.Address, nonce int64) error {
	params := db.UpsertNonceParams{
		Address: addr.Hex(),
		Network: network,
		Nonce:   nonce,
	}

	err := s.db.queries().UpsertNonce(ctx, params)
	if err != nil {
		return fmt.Errorf("upsert nonce: %s", err)
	}

	return nil
}

// ListPendingTx lists all pendings txs.
func (s *SystemStore) ListPendingTx(
	ctx context.Context,
	network string,
	addr common.Address) ([]sqlstore.PendingTx, error) {
	params := db.ListPendingTxParams{
		Address: addr.Hex(),
		Network: network,
	}

	res, err := s.db.queries().ListPendingTx(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list pending tx: %s", err)
	}

	pendingTxs := make([]sqlstore.PendingTx, 0)
	for _, r := range res {
		tx := sqlstore.PendingTx{
			Address:   common.HexToAddress(r.Address),
			Nonce:     r.Nonce,
			Hash:      common.HexToHash(r.Hash),
			Network:   r.Network,
			CreatedAt: r.CreatedAt,
		}

		pendingTxs = append(pendingTxs, tx)
	}

	return pendingTxs, nil
}

// InsertPendingTx insert a new pending tx.
func (s *SystemStore) InsertPendingTx(
	ctx context.Context,
	network string,
	addr common.Address,
	nonce int64, hash common.Hash) error {
	params := db.InsertPendingTxParams{
		Address: addr.Hex(),
		Network: network,
		Nonce:   nonce,
		Hash:    hash.Hex(),
	}

	err := s.db.queries().InsertPendingTx(ctx, params)
	if err != nil {
		return fmt.Errorf("insert pending tx: %s", err)
	}

	return nil
}

// DeletePendingTxByHash deletes a pending tx.
func (s *SystemStore) DeletePendingTxByHash(ctx context.Context, hash common.Hash) error {
	err := s.db.queries().DeletePendingTxByHash(ctx, hash.Hex())
	if err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	return nil
}

// WithTx returns a copy of the current SystemStore with a tx attached.
func (s *SystemStore) WithTx(tx pgx.Tx) sqlstore.SystemStore {
	return &SystemStore{
		&dbWithTxImpl{
			db: s.db.queries(),
			tx: tx,
		},
	}
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

func tableFromSQLToDTO(table db.Registry) (sqlstore.Table, error) {
	br := &big.Rat{}
	if err := table.ID.AssignTo(br); err != nil {
		return sqlstore.Table{}, fmt.Errorf("parsing numeric to bigrat: %s", err)
	}
	if !br.IsInt() {
		return sqlstore.Table{}, errors.New("parsed numeric isn't an integer")
	}
	id, err := tableland.NewTableID(br.Num().String())
	if err != nil {
		return sqlstore.Table{}, fmt.Errorf("parsing id to string: %s", err)
	}
	return sqlstore.Table{
		ID:          id,
		Controller:  table.Controller,
		Name:        table.Name,
		Description: table.Description,
		Structure:   table.Structure,
		CreatedAt:   table.CreatedAt,
	}, nil
}

func aclFromSQLtoDTO(acl db.SystemAcl) (sqlstore.SystemACL, error) {
	br := &big.Rat{}
	if err := acl.TableID.AssignTo(br); err != nil {
		return sqlstore.SystemACL{}, fmt.Errorf("parsing numeric to bigrat: %s", err)
	}
	if !br.IsInt() {
		return sqlstore.SystemACL{}, errors.New("parsed numeric isn't an integer")
	}
	id, err := tableland.NewTableID(br.Num().String())
	if err != nil {
		return sqlstore.SystemACL{}, fmt.Errorf("parsing id to string: %s", err)
	}

	privileges := make(tableland.Privileges, len(acl.Privileges))
	for i, priv := range acl.Privileges {
		privileges[i] = tableland.Privilege(priv)
	}

	systemACL := sqlstore.SystemACL{
		TableID:    id,
		Controller: acl.Controller,
		Privileges: privileges,
		CreatedAt:  acl.CreatedAt,
	}

	if acl.UpdatedAt.Valid {
		systemACL.UpdatedAt = &acl.UpdatedAt.Time
	}

	return systemACL, nil
}

func sanitizeAddress(address string) error {
	if strings.ContainsAny(address, "%_") {
		return errors.New("address contains invalid characters")
	}
	return nil
}

// DBWithTx gives access to db.Queries with the possibility
// of a tx attached, preventing direct access to the db and tx.
type dbWithTx interface {
	queries() *db.Queries
}

type dbWithTxImpl struct {
	db *db.Queries
	tx pgx.Tx
}

func (d *dbWithTxImpl) queries() *db.Queries {
	if d.tx == nil {
		return d.db
	}
	return d.db.WithTx(d.tx)
}
