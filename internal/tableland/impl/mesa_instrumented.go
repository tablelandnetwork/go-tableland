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
	req tableland.CreateTableRequest) (tableland.CreateTableResponse, error) {
	start := time.Now()
	resp, err := t.tableland.CreateTable(ctx, req)
	latency := time.Since(start).Milliseconds()
	t.record(ctx, recordData{"CreateTable", req.Controller, req.ID, err == nil, latency})
	return resp, err
}

// RunSQL allows the user to run SQL.
func (t *InstrumentedTablelandMesa) RunSQL(ctx context.Context,
	req tableland.RunSQLRequest) (tableland.RunSQLResponse, error) {
	start := time.Now()
	resp, err := t.tableland.RunSQL(ctx, req)
	latency := time.Since(start).Milliseconds()

	t.record(ctx, recordData{"RunSQL", req.Controller, "", err == nil, latency})
	return resp, err
}

// Authorize is a convenience API giving the client something to call to trigger authorization.
func (t *InstrumentedTablelandMesa) Authorize(ctx context.Context, req tableland.AuthorizeRequest) error {
	start := time.Now()
	err := t.tableland.Authorize(ctx, req)
	latency := time.Since(start).Milliseconds()
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("Authorize")},
		{Key: "controller", Value: attribute.StringValue(req.Controller)},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
	}
	t.callCount.Add(ctx, 1, attributes...)
	t.latencyHistogram.Record(ctx, latency, attributes...)
	return err
}

func (t *InstrumentedTablelandMesa) record(ctx context.Context, data recordData) {
	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue(data.method)},
		{Key: "controller", Value: attribute.StringValue(data.controller)},
		{Key: "table_id", Value: attribute.StringValue(data.tableID)},
		{Key: "success", Value: attribute.BoolValue(data.success)},
	}

	t.callCount.Add(ctx, 1, attributes...)
	t.latencyHistogram.Record(ctx, data.latency, attributes...)
}
