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
	store    MetricsStore
	exporter MetricsExporter

	nodeID      string
	interval    time.Duration
	fetchAmount int

	quitOnce sync.Once
	quit     chan struct{}
}

// NewPublisher creates a new publisher.
func NewPublisher(s MetricsStore, e MetricsExporter, nodeID string, interval time.Duration) *Publisher {
	return &Publisher{
		store:    s,
		exporter: e,

		nodeID:      nodeID,
		interval:    interval,
		fetchAmount: 100,
		quit:        make(chan struct{}),
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

// Close closes the published goroutine.
func (p *Publisher) Close() {
	p.quitOnce.Do(func() {
		p.quit <- struct{}{}
		close(p.quit)
	})
}

func (p *Publisher) publish(ctx context.Context) error {
	metrics, err := p.store.FetchUnpublishedMetrics(ctx, p.fetchAmount)
	if err != nil {
		return fmt.Errorf("fetch unpublished metrics: %s", err)
	}

	if len(metrics) == 0 {
		return nil
	}

	if err := p.exporter.Export(ctx, metrics, p.nodeID); err != nil {
		return fmt.Errorf("export metrics: %s", err)
	}

	rowsIds := make([]int64, len(metrics))
	for i, m := range metrics {
		rowsIds[i] = m.RowID
	}

	if err := p.store.MarkAsPublished(ctx, rowsIds); err != nil {
		return fmt.Errorf("mark as published: %s", err)
	}

	return nil
}

// MetricsStore defines the API for fetching metrics and marking them as published.
type MetricsStore interface {
	FetchUnpublishedMetrics(context.Context, int) ([]telemetry.Metric, error)
	MarkAsPublished(context.Context, []int64) error
}

// MetricsExporter defines the API for exporting metrics.
type MetricsExporter interface {
	Export(context.Context, []telemetry.Metric, string) error
}
