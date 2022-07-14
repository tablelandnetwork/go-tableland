package counterprobe

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (cp *CounterProbe) initMetrics(chainName string) error {
	meter := global.MeterProvider().Meter("tableland")
	cp.mBaseLabels = []attribute.KeyValue{attribute.String("chain_name", chainName)}

	latencyHistogram, err := meter.SyncInt64().Histogram(metricPrefix + ".latency")
	if err != nil {
		return fmt.Errorf("registering latency histogram: %s", err)
	}
	cp.mLatencyHist = latencyHistogram

	mLastCheck, err := meter.AsyncInt64().Gauge(metricPrefix + ".last_check")
	if err != nil {
		return fmt.Errorf("registering last check gauge: %s", err)
	}

	mLastSuccessfulCheck, err := meter.AsyncInt64().Gauge(metricPrefix + ".last_successful_check")
	if err != nil {
		return fmt.Errorf("registering last full check gauge: %s", err)
	}

	mCounterValue, err := meter.AsyncInt64().Gauge(metricPrefix + ".counter_value")
	if err != nil {
		return fmt.Errorf("registering counter value gauge: %s", err)
	}

	instruments := []instrument.Asynchronous{mLastCheck, mLastSuccessfulCheck, mCounterValue}
	if err := meter.RegisterCallback(instruments, func(ctx context.Context) {
		cp.lock.RLock()
		defer cp.lock.RUnlock()

		if !cp.mLastCheck.IsZero() {
			mLastCheck.Observe(ctx, cp.mLastCheck.Unix(), cp.mBaseLabels...)
		}
		if !cp.mLastSuccessfulCheck.IsZero() {
			mLastSuccessfulCheck.Observe(ctx, cp.mLastSuccessfulCheck.Unix(), cp.mBaseLabels...)
			mCounterValue.Observe(ctx, cp.mLastCounterValue, cp.mBaseLabels...)
		}
	}); err != nil {
		return fmt.Errorf("registering callback on instruments: %s", err)
	}

	return nil
}
