package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// InstrumentedSystemSQLStoreService implements the SystemService interface using SQLStore.
type InstrumentedSystemSQLStoreService struct {
	system           system.SystemService
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

// NewInstrumentedSystemSQLStoreService creates a new InstrumentedSystemSQLStoreService.
func NewInstrumentedSystemSQLStoreService(system system.SystemService) (system.SystemService, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.SyncInt64().Counter("tableland.system.call.count")
	if err != nil {
		return &InstrumentedSystemSQLStoreService{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.system.call.latency")
	if err != nil {
		return &InstrumentedSystemSQLStoreService{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedSystemSQLStoreService{system, callCount, latencyHistogram}, nil
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (s *InstrumentedSystemSQLStoreService) GetTableMetadata(
	ctx context.Context,
	id tableland.TableID,
) (sqlstore.TableMetadata, error) {
	start := time.Now()
	metadata, err := s.system.GetTableMetadata(ctx, id)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTableMetadata")},
		{Key: "id", Value: attribute.StringValue(id.String())},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return metadata, err
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *InstrumentedSystemSQLStoreService) GetTablesByController(ctx context.Context,
	controller string,
) ([]sqlstore.Table, error) {
	start := time.Now()
	tables, err := s.system.GetTablesByController(ctx, controller)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)

	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTablesByController")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}

	s.callCount.Add(ctx, 1, attributes...)
	s.latencyHistogram.Record(ctx, latency, attributes...)

	return tables, err
}
