package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/router/middlewares"
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
	chainID    tableland.ChainID
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

// ValidateCreateTable validates a CREATE TABLE statement and returns its structure hash.
func (t *InstrumentedTablelandMesa) ValidateCreateTable(ctx context.Context,
	req tableland.ValidateCreateTableRequest,
) (tableland.ValidateCreateTableResponse, error) {
	start := time.Now()
	resp, err := t.tableland.ValidateCreateTable(ctx, req)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)
	t.record(ctx, recordData{"ValidateCreateTable", "", "", err == nil, latency, chainID})
	return resp, err
}

// ValidateWriteQuery validates a statement that would mutate a table and returns the table ID.
func (t *InstrumentedTablelandMesa) ValidateWriteQuery(ctx context.Context,
	req tableland.ValidateWriteQueryRequest,
) (tableland.ValidateWriteQueryResponse, error) {
	start := time.Now()
	resp, err := t.tableland.ValidateWriteQuery(ctx, req)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)
	t.record(ctx, recordData{"ValidateWriteQuery", "", "", err == nil, latency, chainID})
	return resp, err
}

// RunReadQuery allows the user to run SQL.
func (t *InstrumentedTablelandMesa) RunReadQuery(ctx context.Context,
	req tableland.RunReadQueryRequest,
) (tableland.RunReadQueryResponse, error) {
	start := time.Now()
	resp, err := t.tableland.RunReadQuery(ctx, req)
	latency := time.Since(start).Milliseconds()

	controller, _ := ctx.Value(middlewares.ContextKeyAddress).(string)
	t.record(ctx, recordData{"RunReadQuery", controller, "", err == nil, latency, 0})
	return resp, err
}

// RelayWriteQuery allows the user to rely on the validator to wrap a write-query in a chain transaction.
func (t *InstrumentedTablelandMesa) RelayWriteQuery(
	ctx context.Context,
	req tableland.RelayWriteQueryRequest,
) (tableland.RelayWriteQueryResponse, error) {
	start := time.Now()
	resp, err := t.tableland.RelayWriteQuery(ctx, req)
	latency := time.Since(start).Milliseconds()

	controller, _ := ctx.Value(middlewares.ContextKeyAddress).(string)
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)
	t.record(ctx, recordData{"RelayWriteQuery", controller, "", err == nil, latency, chainID})
	return resp, err
}

// GetReceipt returns the receipt for a txn hash.
func (t *InstrumentedTablelandMesa) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest,
) (tableland.GetReceiptResponse, error) {
	start := time.Now()
	resp, err := t.tableland.GetReceipt(ctx, req)
	latency := time.Since(start).Milliseconds()

	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)
	t.record(ctx, recordData{"GetReceipt", "", "", err == nil, latency, chainID})
	return resp, err
}

// SetController allows users to the controller for a token id.
func (t *InstrumentedTablelandMesa) SetController(ctx context.Context,
	req tableland.SetControllerRequest,
) (tableland.SetControllerResponse, error) {
	start := time.Now()
	resp, err := t.tableland.SetController(ctx, req)
	latency := time.Since(start).Milliseconds()

	controller, _ := ctx.Value(middlewares.ContextKeyAddress).(string)
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)
	t.record(ctx, recordData{"SetController", controller, "", err == nil, latency, chainID})
	return resp, err
}

func (t *InstrumentedTablelandMesa) record(ctx context.Context, data recordData) {
	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := []attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue(data.method)},
		{Key: "table_id", Value: attribute.StringValue(data.tableID)},
		{Key: "success", Value: attribute.BoolValue(data.success)},
	}

	t.callCount.Add(ctx, 1, attributes...)
	t.latencyHistogram.Record(ctx, data.latency, attributes...)
}
