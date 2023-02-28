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

func (ep *EventProcessor) initMetrics(chainID tableland.ChainID) error {
	meter := global.MeterProvider().Meter("tableland")
	ep.mBaseLabels = append([]attribute.KeyValue{attribute.Int64("chain_id", int64(chainID))}, metrics.BaseAttrs...)

	// Async instruments.
	mExecutionRound, err := meter.Int64ObservableGauge("tableland.eventprocessor.execution.round")
	if err != nil {
		return fmt.Errorf("creating execution round gauge: %s", err)
	}
	mLastProcessedHeight, err := meter.Int64ObservableGauge("tableland.eventprocessor.last.processed.height")
	if err != nil {
		return fmt.Errorf("creating last processed height gauge: %s", err)
	}
	mHashCalculationElapsedTime, err := meter.Int64ObservableGauge("tableland.eventprocessor.hash.calculation.elapsed.time") // nolint
	if err != nil {
		return fmt.Errorf("creating hash calculation elapsed time gauge: %s", err)
	}
	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			o.ObserveInt64(mExecutionRound, ep.mExecutionRound.Load(), ep.mBaseLabels...)
			o.ObserveInt64(mLastProcessedHeight, ep.mLastProcessedHeight.Load(), ep.mBaseLabels...)
			o.ObserveInt64(mHashCalculationElapsedTime, ep.mHashCalculationElapsedTime.Load(), ep.mBaseLabels...)
			return nil
		}, []instrument.Asynchronous{
			mExecutionRound, mLastProcessedHeight, mHashCalculationElapsedTime,
		}...)
	if err != nil {
		return fmt.Errorf("registering async metric callback: %s", err)
	}

	// Sync instruments.
	ep.mEventExecutionCounter, err = meter.Int64Counter("tableland.eventprocessor.event.execution.count")
	if err != nil {
		return fmt.Errorf("creating event execution count instrument: %s", err)
	}
	ep.mTxnExecutionLatency, err = meter.Int64Histogram("tableland.eventprocessor.txn.execution.latency")
	if err != nil {
		return fmt.Errorf("creating txn execution latency instrument: %s", err)
	}
	ep.mBlockExecutionLatency, err = meter.Int64Histogram("tableland.eventprocessor.block.execution.latency")
	if err != nil {
		return fmt.Errorf("creating block execution latency instrument: %s", err)
	}

	return nil
}
