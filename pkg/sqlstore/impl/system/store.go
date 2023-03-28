package system

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/XSAM/otelsql"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/tablelandnetwork/sqlparser"

	"go.opentelemetry.io/otel/attribute"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/sqlite3" // migration for sqlite3
	bindata "github.com/golang-migrate/migrate/v4/source/go_bindata"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/internal/db"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system/migrations"
	"github.com/textileio/go-tableland/pkg/tables"
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
	resolver sqlparser.ReadStatementResolver
}

// New returns a new SystemStore backed by database/sql.
func New(dbURI string, chainID tableland.ChainID) (*SystemStore, error) {
	attrs := append([]attribute.KeyValue{
		attribute.String("name", "systemstore"),
		attribute.Int64("chain_id", int64(chainID)),
	},
		metrics.BaseAttrs...)
	dbc, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
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

// SetReadResolver sets the resolver for read queries.
func (s *SystemStore) SetReadResolver(resolver sqlparser.ReadStatementResolver) {
	s.resolver = resolver
}

// GetTable fetchs a table from its UUID.
func (s *SystemStore) GetTable(ctx context.Context, id tables.TableID) (sqlstore.Table, error) {
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
		ChainID: int64(s.chainID),
		UPPER:   controller,
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
	id tables.TableID,
	controller string,
) (sqlstore.SystemACL, error) {
	params := db.GetAclByTableAndControllerParams{
		ChainID: int64(s.chainID),
		UPPER:   controller,
		TableID: id.ToBigInt().Int64(),
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
			BumpPriceCount: r.BumpPriceCount,
			CreatedAt:      time.Unix(r.CreatedAt, 0),
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
		ChainID: int64(s.chainID),
		Hash:    oldHash.Hex(),
		Hash_2:  newHash.Hex(),
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
	createStmt, err := s.dbWithTx.queries().GetSchemaByTableName(ctx, name)
	if err != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("failed to get the table: %s", err)
	}

	if strings.Contains(strings.ToLower(createStmt), "autoincrement") {
		createStmt = strings.Replace(createStmt, "autoincrement", "", -1)
	}

	index := strings.LastIndex(strings.ToLower(createStmt), "strict")
	ast, err := sqlparser.Parse(createStmt[:index])
	if err != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("failed to parse create stmt: %s", err)
	}

	if ast.Errors[0] != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("non-syntax error: %s", ast.Errors[0])
	}

	createTableNode := ast.Statements[0].(*sqlparser.CreateTable)
	columns := make([]sqlstore.ColumnSchema, len(createTableNode.ColumnsDef))
	for i, col := range createTableNode.ColumnsDef {
		colConstraints := []string{}
		for _, colConstraint := range col.Constraints {
			colConstraints = append(colConstraints, colConstraint.String())
		}

		columns[i] = sqlstore.ColumnSchema{
			Name:        col.Column.String(),
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

// GetID returns node identifier.
func (s *SystemStore) GetID(ctx context.Context) (string, error) {
	id, err := s.dbWithTx.queries().GetId(ctx)
	if err == sql.ErrNoRows {
		id = strings.Replace(uuid.NewString(), "-", "", -1)
		if err := s.dbWithTx.queries().InsertId(ctx, id); err != nil {
			return "", fmt.Errorf("failed to insert id: %s", err)
		}
		return id, nil
	}
	if err != nil {
		return "", fmt.Errorf("failed to get id: %s", err)
	}

	return id, err
}

// Read executes a read statement on the db.
func (s *SystemStore) Read(ctx context.Context, rq parsing.ReadStmt) (*tableland.TableData, error) {
	query, err := rq.GetQuery(s.resolver)
	if err != nil {
		return nil, fmt.Errorf("get query: %s", err)
	}
	ret, err := s.execReadQuery(ctx, s.db, query)
	if err != nil {
		return nil, fmt.Errorf("parsing result to json: %s", err)
	}

	return ret, nil
}

func (s *SystemStore) execReadQuery(ctx context.Context, tx *sql.DB, q string) (*tableland.TableData, error) {
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err = rows.Close(); err != nil {
			s.log.Warn().Err(err).Msg("closing rows")
		}
	}()
	return rowsToTableData(rows)
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
func (s *SystemStore) Begin(_ context.Context) (*sql.Tx, error) {
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
		id, err := tables.NewTableIDFromInt64(res.TableID.Int64)
		if err != nil {
			return eventprocessor.Receipt{}, false, fmt.Errorf("parsing id to string: %s", err)
		}
		receipt.TableID = &id
	}

	return receipt, true, nil
}

// AreEVMEventsPersisted returns true if there're events persisted for the provided txn hash, and false otherwise.
func (s *SystemStore) AreEVMEventsPersisted(ctx context.Context, txnHash common.Hash) (bool, error) {
	params := db.AreEVMEventsPersistedParams{
		ChainID: int64(s.chainID),
		TxHash:  txnHash.Hex(),
	}
	_, err := s.dbWithTx.queries().AreEVMEventsPersisted(ctx, params)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("evm txn events lookup: %s", err)
	}
	return true, nil
}

// SaveEVMEvents saves the provider EVMEvents.
func (s *SystemStore) SaveEVMEvents(ctx context.Context, events []tableland.EVMEvent) error {
	queries := s.dbWithTx.queries()
	for _, e := range events {
		args := db.InsertEVMEventParams{
			ChainID:     int64(e.ChainID),
			EventJson:   string(e.EventJSON),
			EventType:   e.EventType,
			Address:     e.Address.Hex(),
			Topics:      string(e.Topics),
			Data:        e.Data,
			BlockNumber: int64(e.BlockNumber),
			TxHash:      e.TxHash.Hex(),
			TxIndex:     e.TxIndex,
			BlockHash:   e.BlockHash.Hex(),
			EventIndex:  e.Index,
		}
		if err := queries.InsertEVMEvent(ctx, args); err != nil {
			return fmt.Errorf("insert evm event: %s", err)
		}
	}

	return nil
}

// GetBlocksMissingExtraInfo returns a list of block numbers that don't contain enhanced information.
// It receives an optional fromHeight to only look for blocks after a block number. If null it will look
// for blocks at any height.
func (s *SystemStore) GetBlocksMissingExtraInfo(ctx context.Context, lastKnownHeight *int64) ([]int64, error) {
	var blockNumbers []int64
	var err error
	if lastKnownHeight == nil {
		blockNumbers, err = s.dbWithTx.queries().GetBlocksMissingExtraInfo(ctx, int64(s.chainID))
	} else {
		params := db.GetBlocksMissingExtraInfoByBlockNumberParams{
			ChainID:     int64(s.chainID),
			BlockNumber: *lastKnownHeight,
		}
		blockNumbers, err = s.dbWithTx.queries().GetBlocksMissingExtraInfoByBlockNumber(ctx, params)
	}
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get blocks missing extra info: %s", err)
	}

	return blockNumbers, nil
}

// InsertBlockExtraInfo inserts enhanced information for a block.
func (s *SystemStore) InsertBlockExtraInfo(ctx context.Context, blockNumber int64, timestamp uint64) error {
	params := db.InsertBlockExtraInfoParams{
		ChainID:     int64(s.chainID),
		BlockNumber: blockNumber,
		Timestamp:   int64(timestamp),
	}
	if err := s.dbWithTx.queries().InsertBlockExtraInfo(ctx, params); err != nil {
		return fmt.Errorf("insert block extra info: %s", err)
	}

	return nil
}

// GetBlockExtraInfo info returns stored information about an EVM block.
func (s *SystemStore) GetBlockExtraInfo(ctx context.Context, blockNumber int64) (tableland.EVMBlockInfo, error) {
	params := db.GetBlockExtraInfoParams{
		ChainID:     int64(s.chainID),
		BlockNumber: blockNumber,
	}

	blockInfo, err := s.dbWithTx.queries().GetBlockExtraInfo(ctx, params)
	if err == sql.ErrNoRows {
		return tableland.EVMBlockInfo{}, fmt.Errorf("block information not found: %w", err)
	}
	if err != nil {
		return tableland.EVMBlockInfo{}, fmt.Errorf("get block information: %s", err)
	}

	return tableland.EVMBlockInfo{
		ChainID:     tableland.ChainID(blockInfo.ChainID),
		BlockNumber: blockInfo.BlockNumber,
		Timestamp:   time.Unix(blockInfo.Timestamp, 0),
	}, nil
}

// GetEVMEvents returns all the persisted events for a transaction.
func (s *SystemStore) GetEVMEvents(ctx context.Context, txnHash common.Hash) ([]tableland.EVMEvent, error) {
	args := db.GetEVMEventsParams{
		ChainID: int64(s.chainID),
		TxHash:  txnHash.Hex(),
	}
	events, err := s.dbWithTx.queries().GetEVMEvents(ctx, args)
	if err != nil {
		return nil, fmt.Errorf("get events by txhash: %s", err)
	}

	ret := make([]tableland.EVMEvent, len(events))
	for i, event := range events {
		ret[i] = tableland.EVMEvent{
			Address:     common.HexToAddress(event.Address),
			Topics:      []byte(event.Topics),
			Data:        event.Data,
			BlockNumber: uint64(event.BlockNumber),
			TxHash:      common.HexToHash(event.TxHash),
			TxIndex:     event.TxIndex,
			BlockHash:   common.HexToHash(event.BlockHash),
			Index:       event.EventIndex,
			ChainID:     tableland.ChainID(event.ChainID),
			EventJSON:   []byte(event.EventJson),
			EventType:   event.EventType,
		}
	}

	return ret, nil
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
	defer func() {
		if _, err := m.Close(); err != nil {
			s.log.Error().Err(err).Msg("closing db migration")
		}
	}()
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
	id, err := tables.NewTableIDFromInt64(table.ID)
	if err != nil {
		return sqlstore.Table{}, fmt.Errorf("parsing id to string: %s", err)
	}
	return sqlstore.Table{
		ID:         id,
		ChainID:    tableland.ChainID(table.ChainID),
		Controller: table.Controller,
		Prefix:     table.Prefix,
		Structure:  table.Structure,
		CreatedAt:  time.Unix(table.CreatedAt, 0),
	}, nil
}

func aclFromSQLtoDTO(acl db.SystemAcl) (sqlstore.SystemACL, error) {
	id, err := tables.NewTableIDFromInt64(acl.TableID)
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
		CreatedAt:  time.Unix(acl.CreatedAt, 0),
	}

	if acl.UpdatedAt.Valid {
		updatedAt := time.Unix(acl.UpdatedAt.Int64, 0)
		systemACL.UpdatedAt = &updatedAt
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
