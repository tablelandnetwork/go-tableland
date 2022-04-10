package impl

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (ef *EventFeed) initMetrics() error {
	meter := global.MeterProvider().Meter("tableland")

	// Async instruments.
	mHeight, err := meter.AsyncInt64().Gauge("tableland.eventfeed.height")
	if err != nil {
		return fmt.Errorf("creating height gauge: %s", err)
	}
	meter.RegisterCallback([]instrument.Asynchronous{mHeight},
		func(ctx context.Context) {
			mHeight.Observe(ctx, ef.mCurrentHeight.Load())
		})

	// Sync instruments.
	ef.mEventTypeCounter, err = meter.SyncInt64().Counter("tableland.eventfeed.eventypes.count")
	if err != nil {
		return fmt.Errorf("creating event types counter: %s", err)
	}

	return nil
}
