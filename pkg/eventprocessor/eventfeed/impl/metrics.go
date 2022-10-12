package impl

import (
	"context"
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (ef *EventFeed) initMetrics(chainID tableland.ChainID) error {
	meter := global.MeterProvider().Meter("tableland")
	ef.mBaseLabels = append([]attribute.KeyValue{attribute.Int64("chain_id", int64(chainID))}, metrics.BaseAttrs...)

	// Async instruments.
	mHeight, err := meter.AsyncInt64().Gauge("tableland.eventfeed.height")
	if err != nil {
		return fmt.Errorf("creating height gauge: %s", err)
	}
	err = meter.RegisterCallback([]instrument.Asynchronous{mHeight},
		func(ctx context.Context) {
			mHeight.Observe(ctx, ef.mCurrentHeight.Load(), ef.mBaseLabels...)
		})
	if err != nil {
		return fmt.Errorf("registering async callback: %s", err)
	}

	// Sync instruments.
	ef.mEventTypeCounter, err = meter.SyncInt64().Counter("tableland.eventfeed.eventypes.count")
	if err != nil {
		return fmt.Errorf("creating event types counter: %s", err)
	}

	return nil
}
