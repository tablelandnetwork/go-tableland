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
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/metric/unit"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
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

	provider := sdkmetric.NewMeterProvider(sdkmetric.WithReader(exporter))
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

	uptime, err := meter.Int64ObservableGauge(
		"runtime.uptime",
		instrument.WithUnit(unit.Milliseconds),
		instrument.WithDescription("Milliseconds since application was initialized"),
	)
	if err != nil {
		return fmt.Errorf("creating runtime uptime: %s", err)
	}

	goroutines, err := meter.Int64ObservableGauge(
		"process.runtime.go.goroutines",
		instrument.WithDescription("Number of goroutines that currently exist"),
	)
	if err != nil {
		return fmt.Errorf("creating runtime goroutines: %s", err)
	}

	cgoCalls, err := meter.Int64ObservableCounter(
		"process.runtime.go.cgo.calls",
		instrument.WithDescription("Number of cgo calls made by the current process"),
	)
	if err != nil {
		return fmt.Errorf("creating runtime cgo calls: %s", err)
	}

	startTime := time.Now()
	_, err = meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			o.ObserveInt64(uptime, time.Since(startTime).Milliseconds(), BaseAttrs...)
			o.ObserveInt64(goroutines, int64(runtime.NumGoroutine()), BaseAttrs...)
			o.ObserveInt64(cgoCalls, runtime.NumCgoCall(), BaseAttrs...)
			return nil
		},
		[]instrument.Asynchronous{
			uptime,
			goroutines,
			cgoCalls,
		}...,
	)
	if err != nil {
		return fmt.Errorf("registering callback: %s", err)
	}

	return nil
}

func startCollectingMemoryMetrics() error {
	var (
		err error

		heapInuse   instrument.Int64ObservableUpDownCounter
		liveObjects instrument.Int64ObservableUpDownCounter
		gcCount     instrument.Int64ObservableCounter

		lastMemStats time.Time
		memStats     runtime.MemStats
	)

	meter := global.MeterProvider().Meter("runtime")
	if heapInuse, err = meter.Int64ObservableGauge(
		"process.runtime.go.mem.heap_inuse",
		instrument.WithUnit(unit.Bytes),
		instrument.WithDescription("Bytes in in-use spans"),
	); err != nil {
		return fmt.Errorf("creating heap in use: %s", err)
	}

	if liveObjects, err = meter.Int64ObservableGauge(
		"process.runtime.go.mem.live_objects",
		instrument.WithDescription("Number of live objects is the number of cumulative Mallocs - Frees"),
	); err != nil {
		return fmt.Errorf("creating heap live objects: %s", err)
	}

	if gcCount, err = meter.Int64ObservableGauge(
		"process.runtime.go.gc.count",
		instrument.WithDescription("Number of completed garbage collection cycles"),
	); err != nil {
		return fmt.Errorf("creating gc count: %s", err)
	}

	if _, err := meter.RegisterCallback(
		func(ctx context.Context, o metric.Observer) error {
			now := time.Now()
			if now.Sub(lastMemStats) >= time.Second*15 {
				runtime.ReadMemStats(&memStats)
				lastMemStats = now
			}

			o.ObserveInt64(heapInuse, int64(memStats.HeapInuse), BaseAttrs...)
			o.ObserveInt64(liveObjects, int64(memStats.Mallocs-memStats.Frees), BaseAttrs...)
			o.ObserveInt64(gcCount, int64(memStats.NumGC), BaseAttrs...)

			return nil
		}, []instrument.Asynchronous{
			heapInuse,
			liveObjects,
			gcCount,
		}...); err != nil {
		return fmt.Errorf("registering callback: %s", err)
	}
	return nil
}

func aggregatorSelector(ik sdkmetric.InstrumentKind) aggregation.Aggregation {
	switch ik {
	case sdkmetric.InstrumentKindCounter, sdkmetric.InstrumentKindUpDownCounter,
		sdkmetric.InstrumentKindObservableCounter, sdkmetric.InstrumentKindObservableUpDownCounter:
		return aggregation.Sum{}
	case sdkmetric.InstrumentKindObservableGauge:
		return aggregation.LastValue{}
	case sdkmetric.InstrumentKindHistogram:
		return aggregation.ExplicitBucketHistogram{
			Boundaries: []float64{0.5, 1, 2, 4, 10, 50, 100, 500, 1000, 5000},
			NoMinMax:   false,
		}
	}
	panic("unknown instrument kind")
}
