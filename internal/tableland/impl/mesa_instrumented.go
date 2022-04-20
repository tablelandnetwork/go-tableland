package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument/syncint64"
)

// InstrumentedTablelandMesa is the main implementation of Tableland spec with instrumentaion.
type InstrumentedTablelandMesa struct {
	tableland        tableland.Tableland
	callCount        syncint64.Counter
	latencyHistogram syncint64.Histogram
}

type recordData struct {
	method     string
	controller string
	tableID    string
	success    bool
	latency    int64
}

// NewInstrumentedTablelandMesa creates a new InstrumentedTablelandMesa.
func NewInstrumentedTablelandMesa(t tableland.Tableland) (tableland.Tableland, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.SyncInt64().Counter("tableland.mesa.call.count")
	if err != nil {
		return &InstrumentedTablelandMesa{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.SyncInt64().Histogram("tableland.mesa.call.latency")
	if err != nil {
		return &InstrumentedTablelandMesa{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedTablelandMesa{t, callCount, latencyHistogram}, nil
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

// CalculateTableHash allows the user to calculate a table hash.
func (t *InstrumentedTablelandMesa) CalculateTableHash(ctx context.Context,
	req tableland.CalculateTableHashRequest) (tableland.CalculateTableHashResponse, error) {
	start := time.Now()
	resp, err := t.tableland.CalculateTableHash(ctx, req)
	latency := time.Since(start).Milliseconds()
	t.record(ctx, recordData{"CalculateTableHash", "", "", err == nil, latency})
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

func (t *InstrumentedTablelandMesa) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest) (tableland.GetReceiptResponse, error) {
	start := time.Now()
	resp, err := t.tableland.GetReceipt(ctx, req)
	latency := time.Since(start).Milliseconds()

	t.record(ctx, recordData{"GetReceipt", "", "", err == nil, latency})
	return resp, err

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
