package impl

import (
	"context"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
)

// InstrumentedTablelandMesa is the main implementation of Tableland spec with instrumentaion.
type InstrumentedTablelandMesa struct {
	tableland        tableland.Tableland
	callCount        metric.Int64Counter
	latencyHistogram metric.Int64Histogram
}

type recordData struct {
	method     string
	controller string
	tableID    string
	success    bool
	latency    int64
}

// NewInstrumentedTablelandMesa creates a new InstrumentedTablelandMesa.
func NewInstrumentedTablelandMesa(t tableland.Tableland) tableland.Tableland {
	meter := metric.Must(global.Meter("tableland"))
	callCount := meter.NewInt64Counter("tableland.mesa.call.count")
	latencyHistogram := meter.NewInt64Histogram("tableland.mesa.call.latency")

	return &InstrumentedTablelandMesa{t, callCount, latencyHistogram}
}

// CreateTable allows the user to create a table.
func (t *InstrumentedTablelandMesa) CreateTable(ctx context.Context,
	req tableland.Request) (tableland.Response, error) {
	start := time.Now()
	resp, err := t.tableland.CreateTable(ctx, req)
	latency := time.Since(start).Milliseconds()
	t.record(ctx, recordData{"CreateTable", req.Controller, req.TableID, err == nil, latency})
	return resp, err
}

// UpdateTable allows the user to update a table.
func (t *InstrumentedTablelandMesa) UpdateTable(ctx context.Context,
	req tableland.Request) (tableland.Response, error) {
	return t.tableland.UpdateTable(ctx, req)
}

// RunSQL allows the user to run SQL.
func (t *InstrumentedTablelandMesa) RunSQL(ctx context.Context,
	req tableland.Request) (tableland.Response, error) {
	start := time.Now()
	resp, err := t.tableland.RunSQL(ctx, req)
	latency := time.Since(start).Milliseconds()

	t.record(ctx, recordData{"RunSQL", req.Controller, req.TableID, err == nil, latency})
	return resp, err
}

func (t *InstrumentedTablelandMesa) record(ctx context.Context, data recordData) {
	t.callCount.Add(ctx,
		1,
		attribute.KeyValue{Key: "method", Value: attribute.StringValue(data.method)},
		attribute.KeyValue{Key: "controller", Value: attribute.StringValue(data.controller)},
		attribute.KeyValue{Key: "table_id", Value: attribute.StringValue(data.tableID)},
		attribute.KeyValue{Key: "success", Value: attribute.BoolValue(data.success)},
	)

	t.latencyHistogram.Record(ctx,
		data.latency,
		attribute.KeyValue{Key: "method", Value: attribute.StringValue(data.method)},
		attribute.KeyValue{Key: "controller", Value: attribute.StringValue(data.controller)},
		attribute.KeyValue{Key: "table_id", Value: attribute.StringValue(data.tableID)},
		attribute.KeyValue{Key: "success", Value: attribute.BoolValue(data.success)},
	)
}
