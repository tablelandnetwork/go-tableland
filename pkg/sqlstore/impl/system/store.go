package system

import (
	"context"
	"errors"
	"fmt"
	"math/big"

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
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/internal/db"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/migrations"
)

// SystemStore provides a persistent layer for storage requests.
// The methods implemented by this layer can be executed inside a given transaction or not.
// For safety reasons, this layer has no access to the database object or the transaction object.
// The access is made through the dbWithTx interface.
type SystemStore struct {
	chainID tableland.ChainID
	db      dbWithTx
	pool    *pgxpool.Pool
}

// New returns a new SystemStore backed by `pgxpool.Pool`.
func New(pool *pgxpool.Pool, chainID tableland.ChainID) (*SystemStore, error) {
	as := bindata.Resource(migrations.AssetNames(),
		func(name string) ([]byte, error) {
			return migrations.Asset(name)
		})
	err := executeMigration(pool.Config().ConnString(), as)
	if err != nil {
		return nil, fmt.Errorf("initializing db connection: %s", err)
	}

	return &SystemStore{
		db:      &dbWithTxImpl{db: db.New(pool)},
		pool:    pool,
		chainID: chainID,
	}, nil
}

// GetTable fetchs a table from its UUID.
func (s *SystemStore) GetTable(ctx context.Context, id tableland.TableID) (sqlstore.Table, error) {
	dbID := pgtype.Numeric{}
	if err := dbID.Set(id.String()); err != nil {
		return sqlstore.Table{}, fmt.Errorf("parsing id to numeric: %s", err)
	}
	table, err := s.db.queries().GetTable(ctx, db.GetTableParams{
		ChainID: int64(s.chainID),
		ID:      dbID,
	})
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
	sqlcTables, err := s.db.queries().GetTablesByController(ctx, db.GetTablesByControllerParams{
		ChainID:    int64(s.chainID),
		Controller: controller,
	})
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
		ChainID:    int64(s.chainID),
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

// ListPendingTx lists all pendings txs.
func (s *SystemStore) ListPendingTx(ctx context.Context, addr common.Address) ([]nonce.PendingTx, error) {
	params := db.ListPendingTxParams{
		Address: addr.Hex(),
		ChainID: int64(s.chainID),
	}

	res, err := s.db.queries().ListPendingTx(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list pending tx: %s", err)
	}

	pendingTxs := make([]nonce.PendingTx, 0)
	for _, r := range res {
		tx := nonce.PendingTx{
			Address:   common.HexToAddress(r.Address),
			Nonce:     r.Nonce,
			Hash:      common.HexToHash(r.Hash),
			ChainID:   r.ChainID,
			CreatedAt: r.CreatedAt,
		}

		pendingTxs = append(pendingTxs, tx)
	}

	return pendingTxs, nil
}

// InsertPendingTx insert a new pending tx.
func (s *SystemStore) InsertPendingTx(
	ctx context.Context,
	addr common.Address,
	nonce int64, hash common.Hash) error {
	params := db.InsertPendingTxParams{
		Address: addr.Hex(),
		ChainID: int64(s.chainID),
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
	err := s.db.queries().DeletePendingTxByHash(ctx, db.DeletePendingTxByHashParams{
		ChainID: int64(s.chainID),
		Hash:    hash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	return nil
}

// WithTx returns a copy of the current SystemStore with a tx attached.
func (s *SystemStore) WithTx(tx pgx.Tx) sqlstore.SystemStore {
	return &SystemStore{
		chainID: s.chainID,
		db: &dbWithTxImpl{
			db: s.db.queries(),
			tx: tx,
		},
		pool: s.pool,
	}
}

// Begin returns a new tx.
func (s *SystemStore) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.pool.Begin(ctx)
}

// GetReceipt returns a event receipt by transaction hash.
func (s *SystemStore) GetReceipt(
	ctx context.Context,
	txnHash string) (eventprocessor.Receipt, bool, error) {
	params := db.GetReceiptParams{
		ChainID: int64(s.chainID),
		TxnHash: txnHash,
	}

	res, err := s.db.queries().GetReceipt(ctx, params)
	if err == pgx.ErrNoRows {
		return eventprocessor.Receipt{}, false, nil
	}
	if err != nil {
		return eventprocessor.Receipt{}, false, fmt.Errorf("get receipt: %s", err)
	}

	receipt := eventprocessor.Receipt{
		ChainID:     s.chainID,
		BlockNumber: res.BlockNumber,
		TxnHash:     txnHash,
	}
	if res.Error.Valid {
		receipt.Error = &res.Error.String
	}
	if res.TableID.Status == pgtype.Present {
		br := &big.Rat{}
		if err := res.TableID.AssignTo(br); err != nil {
			return eventprocessor.Receipt{}, false, fmt.Errorf("parsing numeric to bigrat: %s", err)
		}
		if !br.IsInt() {
			return eventprocessor.Receipt{}, false, errors.New("parsed numeric isn't an integer")
		}
		id, err := tableland.NewTableID(br.Num().String())
		if err != nil {
			return eventprocessor.Receipt{}, false, fmt.Errorf("parsing id to string: %s", err)
		}
		receipt.TableID = &id
	}

	return receipt, true, nil
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
		ID:         id,
		ChainID:    tableland.ChainID(table.ChainID),
		Controller: table.Controller,
		Prefix:     table.Prefix,
		Structure:  table.Structure,
		CreatedAt:  table.CreatedAt,
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
		ChainID:    tableland.ChainID(acl.ChainID),
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
