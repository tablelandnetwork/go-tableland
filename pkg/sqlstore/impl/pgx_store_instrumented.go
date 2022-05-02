package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// TODO(jsign): chain id metrics.

// InstrumentedSQLStorePGX implements a instrumented SQLStore interface using pgx.
type InstrumentedSQLStorePGX struct {
	chainID          tableland.ChainID
	store            sqlstore.SQLStore
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

// NewInstrumentedSQLStorePGX creates a new pgx pool and instantiate both the user and system stores.
func NewInstrumentedSQLStorePGX(chainID tableland.ChainID, store sqlstore.SQLStore) (sqlstore.SQLStore, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.SyncInt64().Counter("tableland.sqlstore.call.count")
	if err != nil {
		return &InstrumentedSQLStorePGX{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.sqlstore.call.latency")
	if err != nil {
		return &InstrumentedSQLStorePGX{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedSQLStorePGX{
		chainID:          chainID,
		store:            store,
		callCount:        callCount,
		latencyHistogram: latencyHistogram,
	}, nil
}

// GetTable fetchs a table from its UUID.
func (s *InstrumentedSQLStorePGX) GetTable(ctx context.Context, id tableland.TableID) (sqlstore.Table, error) {
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
func (s *InstrumentedSQLStorePGX) GetTablesByController(
	ctx context.Context,
	controller string) ([]sqlstore.Table, error) {
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

// IncrementCreateTableCount increments the counter.
func (s *InstrumentedSQLStorePGX) IncrementCreateTableCount(ctx context.Context, address string) error {
	start := time.Now()
	err := s.store.IncrementCreateTableCount(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("IncrementCreateTableCount")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// IncrementRunSQLCount increments the counter.
func (s *InstrumentedSQLStorePGX) IncrementRunSQLCount(ctx context.Context, address string) error {
	start := time.Now()
	err := s.store.IncrementRunSQLCount(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("IncrementRunSQLCount")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// GetACLOnTableByController increments the counter.
func (s *InstrumentedSQLStorePGX) GetACLOnTableByController(
	ctx context.Context,
	table tableland.TableID,
	address string) (sqlstore.SystemACL, error) {
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

// Read executes a read statement on the db.
func (s *InstrumentedSQLStorePGX) Read(ctx context.Context, stmt parsing.SugaredReadStmt) (interface{}, error) {
	start := time.Now()
	data, err := s.store.Read(ctx, stmt)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("Read")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(s.chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return data, err
}

// ListPendingTx lists all pendings txs.
func (s *InstrumentedSQLStorePGX) ListPendingTx(
	ctx context.Context,
	addr common.Address) ([]nonce.PendingTx, error) {
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
func (s *InstrumentedSQLStorePGX) InsertPendingTx(
	ctx context.Context,
	addr common.Address,
	nonce int64,
	hash common.Hash) error {
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
func (s *InstrumentedSQLStorePGX) DeletePendingTxByHash(ctx context.Context, hash common.Hash) error {
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

// Close closes the connection pool.
func (s *InstrumentedSQLStorePGX) Close() {
	s.store.Close()
}

// WithTx returns a copy of the current InstrumentedSQLStorePGX with a tx attached.
func (s *InstrumentedSQLStorePGX) WithTx(tx pgx.Tx) sqlstore.SystemStore {
	return s.store.WithTx(tx)
}

// Begin returns a new tx.
func (s *InstrumentedSQLStorePGX) Begin(ctx context.Context) (pgx.Tx, error) {
	return s.store.Begin(ctx)
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (s *InstrumentedSQLStorePGX) GetReceipt(
	ctx context.Context,
	txnHash string) (eventprocessor.Receipt, bool, error) {
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
