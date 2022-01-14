package impl

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

// InstrumentedSystemSQLStoreService implements the SystemService interface using SQLStore.
type InstrumentedSystemSQLStoreService struct {
	system           system.SystemService
	callCount        metric.Int64Counter
	latencyHistogram metric.Int64Histogram
}

// NewInstrumentedSystemSQLStoreService creates a new InstrumentedSystemSQLStoreService.
func NewInstrumentedSystemSQLStoreService(system system.SystemService) system.SystemService {
	meter := metric.Must(global.Meter("tableland"))
	callCount := meter.NewInt64Counter("tableland.system.call.count")
	latencyHistogram := meter.NewInt64Histogram("tableland.system.call.latency")

	return &InstrumentedSystemSQLStoreService{system, callCount, latencyHistogram}
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (s *InstrumentedSystemSQLStoreService) GetTableMetadata(ctx context.Context,
	uuid uuid.UUID) (sqlstore.TableMetadata, error) {
	start := time.Now()
	metadata, err := s.system.GetTableMetadata(ctx, uuid)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTableMetadata")},
		{Key: "uuid", Value: attribute.StringValue(uuid.String())},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return metadata, err
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *InstrumentedSystemSQLStoreService) GetTablesByController(ctx context.Context,
	controller string) ([]sqlstore.Table, error) {
	start := time.Now()
	tables, err := s.system.GetTablesByController(ctx, controller)
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
