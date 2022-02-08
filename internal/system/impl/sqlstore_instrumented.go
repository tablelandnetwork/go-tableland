package impl

import (
	"context"
	"time"

	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
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
func (s *InstrumentedSystemSQLStoreService) GetTableMetadata(
	ctx context.Context,
	id tableland.TableID) (sqlstore.TableMetadata, error) {
	start := time.Now()
	metadata, err := s.system.GetTableMetadata(ctx, id)
	latency := time.Since(start).Milliseconds()

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTableMetadata")},
		{Key: "id", Value: attribute.StringValue(id.String())},
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

// Authorize authorizes an address in the SQLStore.
func (s *InstrumentedSystemSQLStoreService) Authorize(ctx context.Context, address string) error {
	start := time.Now()
	err := s.system.Authorize(ctx, address)
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

// Revoke removes an address' access in the SQLStore.
func (s *InstrumentedSystemSQLStoreService) Revoke(ctx context.Context, address string) error {
	start := time.Now()
	err := s.system.Revoke(ctx, address)
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

// IsAuthorized checks the authorization status of an address in the SQLStore.
func (s *InstrumentedSystemSQLStoreService) IsAuthorized(
	ctx context.Context,
	address string,
) (sqlstore.IsAuthorizedResult, error) {
	start := time.Now()
	res, err := s.system.IsAuthorized(ctx, address)
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

// GetAuthorizationRecord gets the authorization record for the provided address from the SQLStore.
func (s *InstrumentedSystemSQLStoreService) GetAuthorizationRecord(
	ctx context.Context,
	address string,
) (sqlstore.AuthorizationRecord, error) {
	start := time.Now()
	record, err := s.system.GetAuthorizationRecord(ctx, address)
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

// ListAuthorized lists all authorization records in the SQLStore.
func (s *InstrumentedSystemSQLStoreService) ListAuthorized(
	ctx context.Context,
) ([]sqlstore.AuthorizationRecord, error) {
	start := time.Now()
	records, err := s.system.ListAuthorized(ctx)
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
