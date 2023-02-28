package impl

import (
	"context"
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (ef *EventFeed) initMetrics(chainID tableland.ChainID) error {
	meter := global.MeterProvider().Meter("tableland")
	ef.mBaseLabels = append([]attribute.KeyValue{attribute.Int64("chain_id", int64(chainID))}, metrics.BaseAttrs...)

	// Async instruments.
	mHeight, err := meter.Int64ObservableGauge("tableland.eventfeed.height")
	if err != nil {
		return fmt.Errorf("creating height gauge: %s", err)
	}
	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			o.ObserveInt64(mHeight, ef.mCurrentHeight.Load(), ef.mBaseLabels...)
			return nil
		}, []instrument.Asynchronous{mHeight}...)
	if err != nil {
		return fmt.Errorf("registering async callback: %s", err)
	}

	// Sync instruments.
	ef.mEventTypeCounter, err = meter.Int64Counter("tableland.eventfeed.eventypes.count")
	if err != nil {
		return fmt.Errorf("creating event types counter: %s", err)
	}

	return nil
}
