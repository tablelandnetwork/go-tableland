package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// InstrumentedSystemStore implements a instrumented SQLStore interface using pgx.
type InstrumentedUserStore struct {
	store            sqlstore.UserStore
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

// NewInstrumentedUserStore creates a new pgx pool and instantiate system stores.
func NewInstrumentedUserStore(store sqlstore.UserStore) (sqlstore.UserStore, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.SyncInt64().Counter("tableland.sqlstore.call.count")
	if err != nil {
		return &InstrumentedUserStore{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.sqlstore.call.latency")
	if err != nil {
		return &InstrumentedUserStore{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedUserStore{
		store:            store,
		callCount:        callCount,
		latencyHistogram: latencyHistogram,
	}, nil
}

// Read executes a read statement on the db.
func (s *InstrumentedUserStore) Read(ctx context.Context, stmt parsing.ReadStmt) (interface{}, error) {
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

// Close closes the store.
func (s *InstrumentedUserStore) Close() error {
	return s.store.Close()
}
