package impl

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (ep *EventProcessor) initMetrics() error {
	meter := global.MeterProvider().Meter("tableland")

	// Async instruments.
	mExecutionRound, err := meter.AsyncInt64().Gauge("tableland.eventprocessor.execution.round")
	if err != nil {
		return fmt.Errorf("creating execution round gauge: %s", err)
	}
	mLastProcessedHeight, err := meter.AsyncInt64().Gauge("tableland.eventprocessor.last.processed.height")
	if err != nil {
		return fmt.Errorf("creating last processed height gauge: %s", err)
	}
	meter.RegisterCallback([]instrument.Asynchronous{mExecutionRound, mLastProcessedHeight},
		func(ctx context.Context) {
			mExecutionRound.Observe(ctx, ep.mExecutionRound.Load())
			mLastProcessedHeight.Observe(ctx, ep.mLastProcessedHeight.Load())
		})

	// Sync instruments.
	ep.mEventExecutionCounter, err = meter.SyncInt64().Counter("tableland.eventprocessor.event.execution.count")
	if err != nil {
		return fmt.Errorf("creating event execution count instrument: %s", err)
	}
	ep.mEventExecutionLatency, err = meter.SyncInt64().Histogram("tableland.eventprocessor.event.execution.latency")
	if err != nil {
		return fmt.Errorf("creating event execution latency instrument: %s", err)
	}
	ep.mBlockExecutionLatency, err = meter.SyncInt64().Histogram("tableland.eventprocessor.block.execution.latency")
	if err != nil {
		return fmt.Errorf("creating block execution latency instrument: %s", err)
	}

	return nil
}
