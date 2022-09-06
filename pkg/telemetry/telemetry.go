package telemetry

import (
	"context"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
)

var (
	c           *collector
	metricStore MetricStore
	log         zerolog.Logger

	mu   = &sync.Mutex{}
	once sync.Once
)

func init() {
	log = logger.With().
		Str("component", "telemetry").
		Logger()
}

// MetricStore specifies the methods for persisting a metric.
type MetricStore interface {
	StoreMetric(context.Context, Metric) error
	Close() error
}

// SetMetricStore sets the store implementation.
// Can only be called once, and should be called before Collect.
func SetMetricStore(s MetricStore) {
	once.Do(func() {
		metricStore = s
	})
}

// Collect collects the metric by persisting locally for later publication.
// If Collect is called before setting the metric store, it will simply log the metric without persisting it.
func Collect(ctx context.Context, metric interface{}) error {
	mu.Lock()
	defer mu.Unlock()
	if metricStore == nil {
		log.Warn().Msg("no metric store was set")
		return nil
	}

	if c == nil {
		c = &collector{
			s: metricStore,
		}
	}

	switch v := metric.(type) {
	case StateHash:
		return c.collect(ctx, Metric{
			Timestamp: time.Now().UTC(),
			Type:      StateHashType,
			Payload: StateHashMetric{
				ChainID:     v.ChainID(),
				BlockNumber: v.BlockNumber(),
				Hash:        v.Hash(),
			},
		})
	default:
		return errors.New("unknown metric")
	}
}

type collector struct {
	s MetricStore
}

func (c *collector) collect(ctx context.Context, metric Metric) error {
	if err := c.s.StoreMetric(ctx, metric); err != nil {
		return errors.Errorf("store metric: %s", err)
	}

	return nil
}
