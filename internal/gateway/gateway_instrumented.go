package gateway

import (
	"context"
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

// InstrumentedGateway implements the Gateway interface using SQLStore.
type InstrumentedGateway struct {
	gateway          Gateway
	callCount        instrument.Int64Counter
	latencyHistogram instrument.Int64Histogram
}

var _ (Gateway) = (*InstrumentedGateway)(nil)

// NewInstrumentedGateway creates a new InstrumentedGateway.
func NewInstrumentedGateway(gateway Gateway) (Gateway, error) {
	meter := global.MeterProvider().Meter("tableland")
	callCount, err := meter.Int64Counter("tableland.system.call.count")
	if err != nil {
		return &InstrumentedGateway{}, fmt.Errorf("registering call counter: %s", err)
	}
	latencyHistogram, err := meter.Int64Histogram("tableland.system.call.latency")
	if err != nil {
		return &InstrumentedGateway{}, fmt.Errorf("registering latency histogram: %s", err)
	}

	return &InstrumentedGateway{gateway, callCount, latencyHistogram}, nil
}

// GetReceiptByTransactionHash implements system.SystemService.
func (g *InstrumentedGateway) GetReceiptByTransactionHash(
	ctx context.Context,
	hash common.Hash,
) (sqlstore.Receipt, bool, error) {
	start := time.Now()
	receipt, exists, err := g.gateway.GetReceiptByTransactionHash(ctx, hash)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetReceiptByTransactionHash")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	g.callCount.Add(ctx, 1, attributes...)
	g.latencyHistogram.Record(ctx, latency, attributes...)

	return receipt, exists, err
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (g *InstrumentedGateway) GetTableMetadata(
	ctx context.Context,
	id tables.TableID,
) (sqlstore.TableMetadata, error) {
	start := time.Now()
	metadata, err := g.gateway.GetTableMetadata(ctx, id)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)

	// NOTE: we may face a risk of high-cardilatity in the future. This should be revised.
	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("GetTableMetadata")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	g.callCount.Add(ctx, 1, attributes...)
	g.latencyHistogram.Record(ctx, latency, attributes...)

	return metadata, err
}

// RunReadQuery allows the user to run SQL.
func (g *InstrumentedGateway) RunReadQuery(ctx context.Context, statement string) (*tableland.TableData, error) {
	start := time.Now()
	data, err := g.gateway.RunReadQuery(ctx, statement)
	latency := time.Since(start).Milliseconds()
	chainID, _ := ctx.Value(middlewares.ContextKeyChainID).(tableland.ChainID)

	attributes := append([]attribute.KeyValue{
		{Key: "method", Value: attribute.StringValue("RunReadQuery")},
		{Key: "success", Value: attribute.BoolValue(err == nil)},
		{Key: "chainID", Value: attribute.Int64Value(int64(chainID))},
	}, metrics.BaseAttrs...)

	g.callCount.Add(ctx, 1, attributes...)
	g.latencyHistogram.Record(ctx, latency, attributes...)

	return data, err
}
