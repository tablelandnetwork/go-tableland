package publisher

import (
	"context"
	"fmt"
	"sync"
	"time"

	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

// Publisher is responsible for fetching unpublished metrics and exporting them.
type Publisher struct {
	fetcher  MetricsFetcher
	exporter MetricsExporter

	interval    time.Duration
	fetchAmount int

	quitOnce sync.Once
	quit     chan struct{}
}

// NewPublisher creates a new publisher.
func NewPublisher(f MetricsFetcher, e MetricsExporter, interval time.Duration) *Publisher {
	return &Publisher{
		fetcher:  f,
		exporter: e,

		interval: interval,
		quit:     make(chan struct{}),
	}
}

var log = logger.With().
	Str("component", "telemetrypublisher").
	Logger()

// Start starts the publisher.
func (p *Publisher) Start() {
	ctx := context.Background()

	ticker := time.NewTicker(p.interval)
	go func() {
		for {
			select {
			case <-ticker.C:
				if err := p.publish(ctx); err != nil {
					log.Err(err).Msg("failed to publish metrics")
				}
			case <-p.quit:
				log.Info().Msg("quiting telemetry publisher")
				ticker.Stop()
				return
			}
		}
	}()
}

// Stop stops the published goroutine.
func (p *Publisher) Stop() {
	p.quitOnce.Do(func() {
		p.quit <- struct{}{}
		close(p.quit)
	})
}

func (p *Publisher) publish(ctx context.Context) error {
	metrics, err := p.fetcher.FetchUnpublishedMetrics(ctx, p.fetchAmount)
	if err != nil {
		return fmt.Errorf("fetch unpublished metrics: %s", err)
	}

	if err := p.exporter.Export(ctx, metrics); err != nil {
		return fmt.Errorf("export metrics: %s", err)
	}

	return nil
}

// MetricsFetcher defines the API for fetching stored metrics.
type MetricsFetcher interface {
	FetchUnpublishedMetrics(context.Context, int) ([]telemetry.Metric, error)
}

// MetricsExporter defines the API for exporting metrics.
type MetricsExporter interface {
	Export(context.Context, []telemetry.Metric) error
}
