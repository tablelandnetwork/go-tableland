package publisher

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

func TestPublisher(t *testing.T) {
	exporter := &MockExporter{}
	fetcher := &fetcher{}
	p := NewPublisher(fetcher, exporter, time.Second)
	p.Start()

	require.Eventually(t, func() bool {
		return exporter.Len() == 1
	}, 5*time.Second, time.Second)

	p.Stop()
}

type fetcher struct{}

func (f *fetcher) FetchUnpublishedMetrics(_ context.Context, _ int) ([]telemetry.Metric, error) {
	return []telemetry.Metric{
		{
			Timestamp: time.Now().UTC(),
			Type:      telemetry.StateHashType,
			Payload: telemetry.StateHashMetric{
				ChainID:     1337,
				BlockNumber: 1,
				Hash:        "abcdef",
			},
		},
	}, nil
}
