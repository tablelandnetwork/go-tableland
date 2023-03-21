package impl

import (
	"context"
	"fmt"
	"time"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/tables"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

// InstrumentedTablelandMesa is the main implementation of Tableland spec with instrumentaion.
type InstrumentedTablelandMesa struct {
	tableland        tableland.Tableland
	callCount        instrument.Int64Counter
	latencyHistogram instrument.Int64Histogram
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
	callCount, err := meter.Int64Counter("tableland.mesa.call.count")
	if err != nil {
		return &InstrumentedTablelandMesa{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.Int64Histogram("tableland.mesa.call.latency")
	if err != nil {
		return &InstrumentedTablelandMesa{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedTablelandMesa{t, callCount, latencyHistogram}, nil
}

// ValidateCreateTable validates a CREATE TABLE statement and returns its structure hash.
func (t *InstrumentedTablelandMesa) ValidateCreateTable(
	ctx context.Context,
	chainID tableland.ChainID,
	stmt string,
) (string, error) {
	start := time.Now()
	resp, err := t.tableland.ValidateCreateTable(ctx, chainID, stmt)
	latency := time.Since(start).Milliseconds()
	t.record(ctx, recordData{"ValidateCreateTable", "", "", err == nil, latency, chainID})
	return resp, err
}

// ValidateWriteQuery validates a statement that would mutate a table and returns the table ID.
func (t *InstrumentedTablelandMesa) ValidateWriteQuery(
	ctx context.Context,
	chainID tableland.ChainID,
	stmt string,
) (tables.TableID, error) {
	start := time.Now()
	resp, err := t.tableland.ValidateWriteQuery(ctx, chainID, stmt)
	latency := time.Since(start).Milliseconds()
	t.record(ctx, recordData{"ValidateWriteQuery", "", "", err == nil, latency, chainID})
	return resp, err
}

// RunReadQuery allows the user to run SQL.
func (t *InstrumentedTablelandMesa) RunReadQuery(ctx context.Context, stmt string) (*tableland.TableData, error) {
	start := time.Now()
	resp, err := t.tableland.RunReadQuery(ctx, stmt)
	latency := time.Since(start).Milliseconds()

	t.record(ctx, recordData{"RunReadQuery", "", "", err == nil, latency, 0})
	return resp, err
}

// GetReceipt returns the receipt for a txn hash.
func (t *InstrumentedTablelandMesa) GetReceipt(
	ctx context.Context,
	chainID tableland.ChainID,
	txnHash string,
) (bool, *tableland.TxnReceipt, error) {
	start := time.Now()
	ok, resp, err := t.tableland.GetReceipt(ctx, chainID, txnHash)
	latency := time.Since(start).Milliseconds()

	t.record(ctx, recordData{"GetReceipt", "", "", err == nil, latency, chainID})
	return ok, resp, err
}

func (t *InstrumentedTablelandMesa) record(ctx context.Context, data recordData) {
	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue(data.method)},
		{Key: "table_id", Value: attribute.StringValue(data.tableID)},
		{Key: "success", Value: attribute.BoolValue(data.success)},
	}, metrics.BaseAttrs...)

	t.callCount.Add(ctx, 1, attributes...)
	t.latencyHistogram.Record(ctx, data.latency, attributes...)
}
