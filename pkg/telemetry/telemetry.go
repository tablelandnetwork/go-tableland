package telemetry

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
)

var (
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
// Only the first call will have an effect. If Collect is called without setting a MetricStore, it will be a noop.
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

	var metricType MetricType
	switch v := metric.(type) {
	case StateHashMetric:
		metricType = StateHashType
	case GitSummaryMetric:
		metricType = GitSummaryType
	case ChainStacksMetric:
		metricType = ChainStacksSummaryType
	case ReadQueryMetric:
		metricType = ReadQueryType
	case NewBlockMetric:
		metricType = NewBlockType
	case NewTablelandEventMetric:
		metricType = NewTablelandEventType
	default:
		return fmt.Errorf("unknown metric type %T", v)
	}
	if err := metricStore.StoreMetric(ctx, Metric{
		Version:   1,
		Timestamp: time.Now().UTC(),
		Type:      metricType,
		Payload:   metric,
	}); err != nil {
		return errors.Errorf("store metric: %s", err)
	}
	return nil
}
