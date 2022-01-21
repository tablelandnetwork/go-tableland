package impl

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

// InstrumentedSQLStorePGX implements a instrumented SQLStore interface using pgx.
type InstrumentedSQLStorePGX struct {
	store            sqlstore.SQLStore
	callCount        metric.Int64Counter
	latencyHistogram metric.Int64Histogram
}

// NewInstrumentedSQLStorePGX creates a new pgx pool and instantiate both the user and system stores.
func NewInstrumentedSQLStorePGX(store sqlstore.SQLStore) sqlstore.SQLStore {
	meter := metric.Must(global.Meter("tableland"))
	callCount := meter.NewInt64Counter("tableland.sqlstore.call.count")
	latencyHistogram := meter.NewInt64Histogram("tableland.sqlstore.call.latency")

	return &InstrumentedSQLStorePGX{store, callCount, latencyHistogram}
}

// InsertTable inserts a new system-wide table.
func (s *InstrumentedSQLStorePGX) InsertTable(ctx context.Context,
	uuid uuid.UUID, controller string, tableType string) error {
	start := time.Now()
	err := s.store.InsertTable(ctx, uuid, controller, tableType)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("InsertTable")},
		{Key: "uuid", Value: attribute.StringValue(uuid.String())},
		{Key: "controller", Value: attribute.StringValue(controller)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// GetTable fetchs a table from its UUID.
func (s *InstrumentedSQLStorePGX) GetTable(ctx context.Context, uuid uuid.UUID) (sqlstore.Table, error) {
	start := time.Now()
	table, err := s.store.GetTable(ctx, uuid)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTable")},
		{Key: "uuid", Value: attribute.StringValue(uuid.String())},
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
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// IsAuthorized checks if the provided address has permission to use the system.
func (s *InstrumentedSQLStorePGX) IsAuthorized(ctx context.Context, address string) (bool, error) {
	start := time.Now()
	authorized, err := s.store.IsAuthorized(ctx, address)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("IsAuthorized")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return authorized, err
}

// Write executes a write statement on the db.
func (s *InstrumentedSQLStorePGX) Write(ctx context.Context, statement string) error {
	start := time.Now()
	err := s.store.Write(ctx, statement)
	latency := time.Since(start).Milliseconds()

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("Write")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return err
}

// Read executes a read statement on the db.
func (s *InstrumentedSQLStorePGX) Read(ctx context.Context, statement string) (interface{}, error) {
	start := time.Now()
	data, err := s.store.Read(ctx, statement)
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
