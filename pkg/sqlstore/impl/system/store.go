package system

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/XSAM/otelsql"
	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/tablelandnetwork/sqlparser"
	"go.opentelemetry.io/otel/attribute"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" // migration for sqlite3
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/mattn/go-sqlite3"
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
	log      zerolog.Logger
	chainID  tableland.ChainID
	dbWithTx dbWithTx
	db       *sql.DB
}

// New returns a new SystemStore backed by database/sql.
func New(dbURI string, chainID tableland.ChainID) (*SystemStore, error) {
	dbc, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(
		attribute.String("name", "systemstore"),
		attribute.Int64("chain_id", int64(chainID)),
	))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	dbc.SetMaxIdleConns(0)
	if err := otelsql.RegisterDBStatsMetrics(dbc, otelsql.WithAttributes(
		attribute.String("name", "systemstore"),
		attribute.Int64("chain_id", int64(chainID)),
	)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}

	log := logger.With().
		Str("component", "systemstore").
		Int64("chain_id", int64(chainID)).
		Logger()

	systemStore := &SystemStore{
		log:      log,
		dbWithTx: &dbWithTxImpl{db: db.New(dbc)},
		db:       dbc,
		chainID:  chainID,
	}

	as := bindata.Resource(migrations.AssetNames(), migrations.Asset)
	if err := systemStore.executeMigration(dbURI, as); err != nil {
		return nil, fmt.Errorf("initializing db connection: %s", err)
	}

	return systemStore, nil
}

// GetTable fetchs a table from its UUID.
func (s *SystemStore) GetTable(ctx context.Context, id tableland.TableID) (sqlstore.Table, error) {
	table, err := s.dbWithTx.queries().GetTable(ctx, db.GetTableParams{
		ChainID: int64(s.chainID),
		ID:      id.ToBigInt().Int64(),
	})
	if err != nil {
		return sqlstore.Table{}, fmt.Errorf("failed to get the table: %w", err)
	}
	return tableFromSQLToDTO(table)
}

// GetTablesByController fetchs a table from controller address.
func (s *SystemStore) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	if err := sanitizeAddress(controller); err != nil {
		return []sqlstore.Table{}, fmt.Errorf("sanitizing address: %s", err)
	}
	sqlcTables, err := s.dbWithTx.queries().GetTablesByController(ctx, db.GetTablesByControllerParams{
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
	controller string,
) (sqlstore.SystemACL, error) {
	params := db.GetAclByTableAndControllerParams{
		ChainID:    int64(s.chainID),
		Controller: controller,
		TableID:    id.ToBigInt().Int64(),
	}

	systemACL, err := s.dbWithTx.queries().GetAclByTableAndController(ctx, params)
	if err == sql.ErrNoRows {
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

	res, err := s.dbWithTx.queries().ListPendingTx(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list pending tx: %s", err)
	}

	pendingTxs := make([]nonce.PendingTx, 0)
	for _, r := range res {
		tx := nonce.PendingTx{
			Address:        common.HexToAddress(r.Address),
			Nonce:          r.Nonce,
			Hash:           common.HexToHash(r.Hash),
			ChainID:        r.ChainID,
			BumpPriceCount: int(r.BumpPriceCount),
			CreatedAt:      r.CreatedAt,
		}

		pendingTxs = append(pendingTxs, tx)
	}

	return pendingTxs, nil
}

// InsertPendingTx insert a new pending tx.
func (s *SystemStore) InsertPendingTx(
	ctx context.Context,
	addr common.Address,
	nonce int64, hash common.Hash,
) error {
	params := db.InsertPendingTxParams{
		Address: addr.Hex(),
		ChainID: int64(s.chainID),
		Nonce:   nonce,
		Hash:    hash.Hex(),
	}

	err := s.dbWithTx.queries().InsertPendingTx(ctx, params)
	if err != nil {
		return fmt.Errorf("insert pending tx: %s", err)
	}

	return nil
}

// DeletePendingTxByHash deletes a pending tx.
func (s *SystemStore) DeletePendingTxByHash(ctx context.Context, hash common.Hash) error {
	err := s.dbWithTx.queries().DeletePendingTxByHash(ctx, db.DeletePendingTxByHashParams{
		ChainID: int64(s.chainID),
		Hash:    hash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("delete pending tx: %s", err)
	}

	return nil
}

// ReplacePendingTxByHash replaces the txn hash of a pending txn and bumps the counter of how many times this happened.
func (s *SystemStore) ReplacePendingTxByHash(ctx context.Context, oldHash common.Hash, newHash common.Hash) error {
	err := s.dbWithTx.queries().ReplacePendingTxByHash(ctx, db.ReplacePendingTxByHashParams{
		ChainID:      int64(s.chainID),
		PreviousHash: oldHash.Hex(),
		NewHash:      newHash.Hex(),
	})
	if err != nil {
		return fmt.Errorf("replace pending tx: %s", err)
	}
	return nil
}

// GetTablesByStructure gets all tables with a particular structure hash.
func (s *SystemStore) GetTablesByStructure(ctx context.Context, structure string) ([]sqlstore.Table, error) {
	rows, err := s.dbWithTx.queries().GetTablesByStructure(ctx, db.GetTablesByStructureParams{
		ChainID:   int64(s.chainID),
		Structure: structure,
	})
	if err != nil {
		return []sqlstore.Table{}, fmt.Errorf("failed to get the table: %s", err)
	}

	tables := make([]sqlstore.Table, len(rows))
	for i := range rows {
		tables[i], err = tableFromSQLToDTO(rows[i])
		if err != nil {
			return nil, fmt.Errorf("parsing database table to dto: %s", err)
		}
	}

	return tables, nil
}

// GetSchemaByTableName get the schema of a table by its name.
func (s *SystemStore) GetSchemaByTableName(ctx context.Context, name string) (sqlstore.TableSchema, error) {
	createStmt, err := s.dbWithTx.queries().GetSchemaByTableName(ctx, db.GetSchemaByTableNameParams{
		TableName: name,
	})
	if err != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("failed to get the table: %s", err)
	}

	index := strings.LastIndex(createStmt, "STRICT")
	ast, err := sqlparser.Parse(createStmt[:index])
	if err != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("failed to parse create stmt: %s", err)
	}

	if ast.Errors[0] != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("non-syntax error: %s", ast.Errors[0])
	}

	createTableNode := ast.Statements[0].(*sqlparser.CreateTable)
	columns := make([]sqlstore.ColumnSchema, len(createTableNode.Columns))
	for i, col := range createTableNode.Columns {
		colConstraints := []string{}
		for _, colConstraint := range col.Constraints {
			colConstraints = append(colConstraints, colConstraint.String())
		}

		columns[i] = sqlstore.ColumnSchema{
			Name:        col.Name.String(),
			Type:        strings.ToLower(col.Type),
			Constraints: colConstraints,
		}
	}

	tableConstraints := make([]string, len(createTableNode.Constraints))
	for i, tableConstraint := range createTableNode.Constraints {
		tableConstraints[i] = tableConstraint.String()
	}

	return sqlstore.TableSchema{
		Columns:          columns,
		TableConstraints: tableConstraints,
	}, nil
}

// WithTx returns a copy of the current SystemStore with a tx attached.
func (s *SystemStore) WithTx(tx *sql.Tx) sqlstore.SystemStore {
	return &SystemStore{
		chainID: s.chainID,
		dbWithTx: &dbWithTxImpl{
			db: s.dbWithTx.queries(),
			tx: tx,
		},
		db: s.db,
	}
}

// Begin returns a new tx.
func (s *SystemStore) Begin(ctx context.Context) (*sql.Tx, error) {
	return s.db.Begin()
}

// GetReceipt returns a event receipt by transaction hash.
func (s *SystemStore) GetReceipt(
	ctx context.Context,
	txnHash string,
) (eventprocessor.Receipt, bool, error) {
	params := db.GetReceiptParams{
		ChainID: int64(s.chainID),
		TxnHash: txnHash,
	}

	res, err := s.dbWithTx.queries().GetReceipt(ctx, params)
	if err == sql.ErrNoRows {
		return eventprocessor.Receipt{}, false, nil
	}
	if err != nil {
		return eventprocessor.Receipt{}, false, fmt.Errorf("get receipt: %s", err)
	}

	receipt := eventprocessor.Receipt{
		ChainID:      s.chainID,
		BlockNumber:  res.BlockNumber,
		IndexInBlock: res.IndexInBlock,
		TxnHash:      txnHash,
	}
	if res.Error.Valid {
		receipt.Error = &res.Error.String

		errorEventIdx := int(res.ErrorEventIdx.Int64)
		receipt.ErrorEventIdx = &errorEventIdx
	}
	if res.TableID.Valid {
		id, err := tableland.NewTableIDFromInt64(res.TableID.Int64)
		if err != nil {
			return eventprocessor.Receipt{}, false, fmt.Errorf("parsing id to string: %s", err)
		}
		receipt.TableID = &id
	}

	return receipt, true, nil
}

func (s *SystemStore) AreEVMEventsPersisted(ctx context.Context, txnHash common.Hash) (bool, error) {
	params := db.AreEVMTxnEventsPersistedParams{
		ChainID: uint64(s.chainID),
		TxHash:  txnHash.Hex(),
	}
	ok, err := s.dbWithTx.queries().AreEVMEventsPersisted(ctx, params)
	if err != nil {
		return false, fmt.Errorf("evm txn events lookup: %s", err)
	}
	return ok, nil
}

func (s *SystemStore) SaveEVMEvents(ctx context.Context, events []tableland.EVMEvent) error {
	queries := s.dbWithTx.queries()
	for _, e := range events {
		args := db.InsertEVMEventParams{
			ChainID:     uint64(e.ChainID),
			EventJSON:   e.EventJSON,
			Address:     e.Address.Hex(),
			Topics:      e.Topics,
			Data:        e.Data,
			BlockNumber: e.BlockNumber,
			TxHash:      e.TxHash.Hex(),
			TxIndex:     e.TxIndex,
			BlockHash:   e.BlockHash.Hex(),
			Index:       e.Index,
		}
		if err := queries.InsertEVMEvent(ctx, args); err != nil {
			var sqlErr sqlite3.Error
			if errors.As(err, &sqlErr) {
				if sqlErr.ExtendedCode == sqlite3.ErrConstraintPrimaryKey {
					s.log.Warn().Str("txn_hash", e.TxHash.Hex()).Msg("event was already stored")
					continue
				}
				return fmt.Errorf("insert evm event: %s", err)
			}
		}
	}

	return nil
}

// Close closes the store.
func (s *SystemStore) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing db: %s", err)
	}
	return nil
}

// executeMigration run db migrations and return a ready to use connection to the SQLite database.
func (s *SystemStore) executeMigration(dbURI string, as *bindata.AssetSource) error {
	d, err := bindata.WithInstance(as)
	if err != nil {
		return fmt.Errorf("creating source driver: %s", err)
	}

	m, err := migrate.NewWithSourceInstance("go-bindata", d, "sqlite3://"+dbURI)
	if err != nil {
		return fmt.Errorf("creating migration: %s", err)
	}
	version, dirty, err := m.Version()
	s.log.Info().
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
	id, err := tableland.NewTableIDFromInt64(table.ID)
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
	id, err := tableland.NewTableIDFromInt64(acl.TableID)
	if err != nil {
		return sqlstore.SystemACL{}, fmt.Errorf("parsing id to string: %s", err)
	}

	var privileges tableland.Privileges
	if acl.Privileges&tableland.PrivInsert.Bitfield > 0 {
		privileges = append(privileges, tableland.PrivInsert)
	}
	if acl.Privileges&tableland.PrivUpdate.Bitfield > 0 {
		privileges = append(privileges, tableland.PrivUpdate)
	}
	if acl.Privileges&tableland.PrivDelete.Bitfield > 0 {
		privileges = append(privileges, tableland.PrivDelete)
	}

	systemACL := sqlstore.SystemACL{
		ChainID:    tableland.ChainID(acl.ChainID),
		TableID:    id,
		Controller: acl.Controller,
		Privileges: privileges,
		CreatedAt:  acl.CreatedAt,
		UpdatedAt:  acl.UpdatedAt,
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
	tx *sql.Tx
}

func (d *dbWithTxImpl) queries() *db.Queries {
	if d.tx == nil {
		return d.db
	}
	return d.db.WithTx(d.tx)
}
