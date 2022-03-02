package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// InstrumentedSQLStorePGX implements a instrumented SQLStore interface using pgx.
type InstrumentedSQLStorePGX struct {
	store            sqlstore.SQLStore
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

// NewInstrumentedSQLStorePGX creates a new pgx pool and instantiate both the user and system stores.
func NewInstrumentedSQLStorePGX(store sqlstore.SQLStore) (sqlstore.SQLStore, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.SyncInt64().Counter("tableland.sqlstore.call.count")
	if err != nil {
		return &InstrumentedSQLStorePGX{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.sqlstore.call.latency")
	if err != nil {
		return &InstrumentedSQLStorePGX{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedSQLStorePGX{store, callCount, latencyHistogram}, nil
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
	}
	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return table, err
}

// GetTablesByController fetchs a table from controller address.
func (s *InstrumentedSQLStorePGX) GetTablesByController(ctx context.Context,
	controller string) ([]sqlstore.Table, error) {
	start := time.Now()
	tables, err := s.store.GetTablesByController(ctx, controller)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTablesByController")},
		{Key: "controller", Value: attribute.StringValue(controller)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return tables, err
}

// Authorize grants the provided address permission to use the system.
func (s *InstrumentedSQLStorePGX) Authorize(ctx context.Context, address string) error {
	start := time.Now()
	err := s.store.Authorize(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("Authorize")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// Revoke removes permission to use the system from the provided address.
func (s *InstrumentedSQLStorePGX) Revoke(ctx context.Context, address string) error {
	start := time.Now()
	err := s.store.Revoke(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("Revoke")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// IsAuthorized checks if the provided address has permission to use the system.
func (s *InstrumentedSQLStorePGX) IsAuthorized(
	ctx context.Context,
	address string,
) (sqlstore.IsAuthorizedResult, error) {
	start := time.Now()
	res, err := s.store.IsAuthorized(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("IsAuthorized")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return res, err
}

// GetAuthorizationRecord gets the authorization record for the provided address.
func (s *InstrumentedSQLStorePGX) GetAuthorizationRecord(
	ctx context.Context,
	address string,
) (sqlstore.AuthorizationRecord, error) {
	start := time.Now()
	record, err := s.store.GetAuthorizationRecord(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetAuthorizationRecord")},
		{Key: "address", Value: attribute.StringValue(address)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return record, err
}

// ListAuthorized returns a list of all authorization records.
func (s *InstrumentedSQLStorePGX) ListAuthorized(ctx context.Context) ([]sqlstore.AuthorizationRecord, error) {
	start := time.Now()
	records, err := s.store.ListAuthorized(ctx)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("ListAuthorized")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return records, err
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
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return data, err
}

// Close closes the connection pool.
func (s *InstrumentedSQLStorePGX) Close() {
	s.store.Close()
}
