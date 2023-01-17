package metrics

import (
	"context"
	"fmt"
	"net/http"
	"runtime"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.opentelemetry.io/otel/attribute"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric/unit"

	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/instrument/asyncint64"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/metric/aggregation"
)

// BaseAttrs contains attributes that should be added in all exported metrics.
var BaseAttrs []attribute.KeyValue

// SetupInstrumentation starts a metric endpoint.
func SetupInstrumentation(prometheusAddr string, serviceName string) error {
	BaseAttrs = []attribute.KeyValue{attribute.String("service_name", serviceName)}

	exporter, err := otelprom.New(otelprom.WithAggregationSelector(aggregatorSelector))
	if err != nil {
		return fmt.Errorf("creating prometheus exporter: %s", err)
	}

	provider := metric.NewMeterProvider(metric.WithReader(exporter))
	global.SetMeterProvider(provider)

	http.Handle("/metrics", promhttp.Handler())
	go func() {
		_ = http.ListenAndServe(prometheusAddr, nil)
	}()

	if err := startCollectingRuntimeMetrics(); err != nil {
		return fmt.Errorf("start collecting Go runtime metrics: %s", err)
	}

	if err := startCollectingMemoryMetrics(); err != nil {
		return fmt.Errorf("start collecting Go memory metrics: %s", err)
	}

	return nil
}

func startCollectingRuntimeMetrics() error {
	meter := global.MeterProvider().Meter("runtime")

	uptime, err := meter.AsyncInt64().Gauge(
		"runtime.uptime",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Milliseconds since application was initialized"),
	)
	if err != nil {
		return fmt.Errorf("creating runtime uptime: %s", err)
	}

	goroutines, err := meter.AsyncInt64().Gauge(
		"process.runtime.go.goroutines",
		instrument.WithDescription("Number of goroutines that currently exist"),
	)
	if err != nil {
		return fmt.Errorf("creating runtime goroutines: %s", err)
	}

	cgoCalls, err := meter.AsyncInt64().Counter(
		"process.runtime.go.cgo.calls",
		instrument.WithDescription("Number of cgo calls made by the current process"),
	)
	if err != nil {
		return fmt.Errorf("creating runtime cgo calls: %s", err)
	}

	startTime := time.Now()
	err = meter.RegisterCallback(
		[]instrument.Asynchronous{
			uptime,
			goroutines,
			cgoCalls,
		},
		func(ctx context.Context) {
			uptime.Observe(ctx, time.Since(startTime).Milliseconds(), BaseAttrs...)
			goroutines.Observe(ctx, int64(runtime.NumGoroutine()), BaseAttrs...)
			cgoCalls.Observe(ctx, runtime.NumCgoCall(), BaseAttrs...)
		},
	)
	if err != nil {
		return fmt.Errorf("registering callback: %s", err)
	}

	return nil
}

func startCollectingMemoryMetrics() error {
	var (
		err error

		heapInuse   asyncint64.UpDownCounter
		liveObjects asyncint64.UpDownCounter
		gcCount     asyncint64.Counter

		lastMemStats time.Time
		memStats     runtime.MemStats
	)

	meter := global.MeterProvider().Meter("runtime")
	if heapInuse, err = meter.AsyncInt64().Gauge(
		"process.runtime.go.mem.heap_inuse",
		instrument.WithUnit(unit.Bytes),
		instrument.WithDescription("Bytes in in-use spans"),
	); err != nil {
		return fmt.Errorf("creating heap in use: %s", err)
	}

	if liveObjects, err = meter.AsyncInt64().Gauge(
		"process.runtime.go.mem.live_objects",
		instrument.WithDescription("Number of live objects is the number of cumulative Mallocs - Frees"),
	); err != nil {
		return fmt.Errorf("creating heap live objects: %s", err)
	}

	if gcCount, err = meter.AsyncInt64().Gauge(
		"process.runtime.go.gc.count",
		instrument.WithDescription("Number of completed garbage collection cycles"),
	); err != nil {
		return fmt.Errorf("creating gc count: %s", err)
	}

	if err := meter.RegisterCallback(
		[]instrument.Asynchronous{
			heapInuse,
			liveObjects,
			gcCount,
		}, func(ctx context.Context) {
			now := time.Now()
			if now.Sub(lastMemStats) >= time.Second*15 {
				runtime.ReadMemStats(&memStats)
				lastMemStats = now
			}

			heapInuse.Observe(ctx, int64(memStats.HeapInuse), BaseAttrs...)
			liveObjects.Observe(ctx, int64(memStats.Mallocs-memStats.Frees), BaseAttrs...)
			gcCount.Observe(ctx, int64(memStats.NumGC), BaseAttrs...)
		}); err != nil {
		return fmt.Errorf("registering callback: %s", err)
	}
	return nil
}

func aggregatorSelector(ik metric.InstrumentKind) aggregation.Aggregation {
	switch ik {
	case metric.InstrumentKindSyncCounter, metric.InstrumentKindSyncUpDownCounter,
		metric.InstrumentKindAsyncCounter, metric.InstrumentKindAsyncUpDownCounter:
		return aggregation.Sum{}
	case metric.InstrumentKindAsyncGauge:
		return aggregation.LastValue{}
	case metric.InstrumentKindSyncHistogram:
		return aggregation.ExplicitBucketHistogram{
			Boundaries: []float64{0.5, 1, 2, 4, 10, 50, 100, 500, 1000, 5000},
			NoMinMax:   false,
		}
	}
	panic("unknown instrument kind")
}
