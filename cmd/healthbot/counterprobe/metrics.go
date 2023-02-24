package counterprobe

import (
	"context"
	"fmt"

	"github.com/textileio/go-tableland/pkg/metrics"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
)

func (cp *CounterProbe) initMetrics(chainName string) error {
	meter := global.MeterProvider().Meter("tableland")
	cp.mBaseLabels = append([]attribute.KeyValue{attribute.String("chain_name", chainName)}, metrics.BaseAttrs...)

	latencyHistogram, err := meter.Int64Histogram(metricPrefix + ".latency")
	if err != nil {
		return fmt.Errorf("registering latency histogram: %s", err)
	}
	cp.mLatencyHist = latencyHistogram

	mLastCheck, err := meter.Int64ObservableGauge(metricPrefix + ".last_check")
	if err != nil {
		return fmt.Errorf("registering last check gauge: %s", err)
	}

	mLastSuccessfulCheck, err := meter.Int64ObservableGauge(metricPrefix + ".last_successful_check")
	if err != nil {
		return fmt.Errorf("registering last full check gauge: %s", err)
	}

	mCounterValue, err := meter.Int64ObservableGauge(metricPrefix + ".counter_value")
	if err != nil {
		return fmt.Errorf("registering counter value gauge: %s", err)
	}

	instruments := []instrument.Asynchronous{mLastCheck, mLastSuccessfulCheck, mCounterValue}
	if _, err := meter.RegisterCallback(func(ctx context.Context, o metric.Observer) error {
		cp.lock.RLock()
		defer cp.lock.RUnlock()

		if !cp.mLastCheck.IsZero() {
			o.ObserveInt64(mLastCheck, cp.mLastCheck.Unix(), cp.mBaseLabels...)
		}
		if !cp.mLastSuccessfulCheck.IsZero() {
			o.ObserveInt64(mLastSuccessfulCheck, cp.mLastSuccessfulCheck.Unix(), cp.mBaseLabels...)
			o.ObserveInt64(mCounterValue, cp.mLastCounterValue, cp.mBaseLabels...)
		}
		return nil
	}, instruments...); err != nil {
		return fmt.Errorf("registering callback on instruments: %s", err)
	}

	return nil
}
