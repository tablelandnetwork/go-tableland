package impl

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// InstrumentedSystemStore implements a instrumented SQLStore.
type InstrumentedSystemStore struct {
	chainID          tableland.ChainID
	store            sqlstore.SystemStore
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

// NewInstrumentedSystemStore creates a new db pool and instantiate both the user and system stores.
func NewInstrumentedSystemStore(chainID tableland.ChainID, store sqlstore.SystemStore) (sqlstore.SystemStore, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.SyncInt64().Counter("tableland.sqlstore.call.count")
	if err != nil {
		return &InstrumentedSystemStore{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.sqlstore.call.latency")
	if err != nil {
		return &InstrumentedSystemStore{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedSystemStore{
		chainID:          chainID,
		store:            store,
		callCount:        callCount,
		latencyHistogram: latencyHistogram,
	}, nil
}

// GetTable fetchs a table from its UUID.
func (s *InstrumentedSystemStore) GetTable(ctx context.Context, id tables.TableID) (sqlstore.Table, error) {
	start := time.Now()
	table, err := s.store.GetTable(ctx, id)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTable")},
		{Key: "id", Value: attribute.StringValue(id.String())},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}
	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return table, err
}

// GetTablesByController fetchs a table from controller address.
func (s *InstrumentedSystemStore) GetTablesByController(
	ctx context.Context,
	controller string,
) ([]sqlstore.Table, error) {
	start := time.Now()
	tables, err := s.store.GetTablesByController(ctx, controller)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTablesByController")},
		{Key: "controller", Value: attribute.StringValue(controller)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return tables, err
}

// GetTablesByStructure gets all tables with a particular structure hash.
func (s *InstrumentedSystemStore) GetTablesByStructure(
	ctx context.Context,
	structure string,
) ([]sqlstore.Table, error) {
	start := time.Now()
	tables, err := s.store.GetTablesByStructure(ctx, structure)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTablesByStructure")},
		{Key: "structure", Value: attribute.StringValue(structure)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return tables, err
}

// GetSchemaByTableName get the schema of a table by its name.
func (s *InstrumentedSystemStore) GetSchemaByTableName(ctx context.Context, name string) (sqlstore.TableSchema, error) {
	start := time.Now()
	tables, err := s.store.GetSchemaByTableName(ctx, name)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetSchemaByTableName")},
		{Key: "name", Value: attribute.StringValue(name)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return tables, err
}

// GetACLOnTableByController increments the counter.
func (s *InstrumentedSystemStore) GetACLOnTableByController(
	ctx context.Context,
	table tables.TableID,
	address string,
) (sqlstore.SystemACL, error) {
	start := time.Now()
	systemACL, err := s.store.GetACLOnTableByController(ctx, table, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetACLOnTableByController")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return systemACL, err
}

// ListPendingTx lists all pendings txs.
func (s *InstrumentedSystemStore) ListPendingTx(
	ctx context.Context,
	addr common.Address,
) ([]nonce.PendingTx, error) {
	start := time.Now()
	data, err := s.store.ListPendingTx(ctx, addr)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ListPendingTx")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return data, err
}

// InsertPendingTx insert a new pending tx.
func (s *InstrumentedSystemStore) InsertPendingTx(
	ctx context.Context,
	addr common.Address,
	nonce int64,
	hash common.Hash,
) error {
	start := time.Now()
	err := s.store.InsertPendingTx(ctx, addr, nonce, hash)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("InsertPendingTx")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// DeletePendingTxByHash deletes a pending tx.
func (s *InstrumentedSystemStore) DeletePendingTxByHash(ctx context.Context, hash common.Hash) error {
	start := time.Now()
	err := s.store.DeletePendingTxByHash(ctx, hash)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("DeletePendingTxByHash")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// ReplacePendingTxByHash replaces a pending txn hash and bumps the counter on how many times this happened.
func (s *InstrumentedSystemStore) ReplacePendingTxByHash(
	ctx context.Context,
	oldHash common.Hash,
	newHash common.Hash,
) error {
	start := time.Now()
	err := s.store.ReplacePendingTxByHash(ctx, oldHash, newHash)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ReplacePendingTxByHash")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// Close closes the connection pool.
func (s *InstrumentedSystemStore) Close() error {
	return s.store.Close()
}

// WithTx returns a copy of the current InstrumentedSQLStore with a tx attached.
func (s *InstrumentedSystemStore) WithTx(tx *sql.Tx) sqlstore.SystemStore {
	return s.store.WithTx(tx)
}

// Begin returns a new tx.
func (s *InstrumentedSystemStore) Begin(ctx context.Context) (*sql.Tx, error) {
	return s.store.Begin(ctx)
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (s *InstrumentedSystemStore) GetReceipt(
	ctx context.Context,
	txnHash string,
) (eventprocessor.Receipt, bool, error) {
	log.Debug().Str("txn_hash", txnHash).Msg("call GetReceipt")
	start := time.Now()
	receipt, ok, err := s.store.GetReceipt(ctx, txnHash)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetReceipt")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return receipt, ok, err
}

// AreEVMEventsPersisted implements sqlstore.SystemStore.
func (s *InstrumentedSystemStore) AreEVMEventsPersisted(ctx context.Context, txnHash common.Hash) (bool, error) {
	log.Debug().Str("txn_hash", txnHash.Hex()).Msg("call AreEVMEventsPersisted")
	start := time.Now()
	ok, err := s.store.AreEVMEventsPersisted(ctx, txnHash)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("AreEVMEventsPersisted")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return ok, err
}

// SaveEVMEvents implements sqlstore.SystemStore.
func (s *InstrumentedSystemStore) SaveEVMEvents(ctx context.Context, events []tableland.EVMEvent) error {
	log.Debug().Msg("call SaveEVMEvents")
	start := time.Now()
	err := s.store.SaveEVMEvents(ctx, events)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("SaveEVMEvents")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// GetEVMEvents implements sqlstore.SystemStore.
func (s *InstrumentedSystemStore) GetEVMEvents(ctx context.Context, txnHash common.Hash) ([]tableland.EVMEvent, error) {
	log.Debug().Str("txn_hash", txnHash.Hex()).Msg("call GetEVMEvents")
	start := time.Now()
	events, err := s.store.GetEVMEvents(ctx, txnHash)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetEVMEvents")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return events, err
}

// GetBlocksMissingExtraInfo implements sqlstore.SystemStore.
func (s *InstrumentedSystemStore) GetBlocksMissingExtraInfo(
	ctx context.Context,
	fromHeight *int64,
) ([]int64, error) {
	start := time.Now()
	blockNumbers, err := s.store.GetBlocksMissingExtraInfo(ctx, fromHeight)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetBlocksMissingExtraInfo")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return blockNumbers, err
}

// GetBlockExtraInfo implements sqlstore.SystemStore.
func (s *InstrumentedSystemStore) GetBlockExtraInfo(
	ctx context.Context,
	blockNumber int64,
) (tableland.EVMBlockInfo, error) {
	log.Debug().Msg("call GetBlockExtraInfo")
	start := time.Now()
	blockInfo, err := s.store.GetBlockExtraInfo(ctx, blockNumber)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetBlockExtraInfo")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return blockInfo, err
}

// InsertBlockExtraInfo implements sqlstore.SystemStore.
func (s *InstrumentedSystemStore) InsertBlockExtraInfo(ctx context.Context, blockNumber int64, timestamp uint64) error {
	start := time.Now()
	err := s.store.InsertBlockExtraInfo(ctx, blockNumber, timestamp)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("InsertBlockExtraInfo")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}
