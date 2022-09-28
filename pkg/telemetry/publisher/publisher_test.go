package publisher

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

func TestPublisher(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	exporter, err := NewHTTPExporter(ts.URL)
	require.NoError(t, err)
	store := newStore()

	nodeID := strings.Replace(uuid.NewString(), "-", "", -1)
	p := NewPublisher(store, exporter, nodeID, time.Second)
	p.Start()

	require.Eventually(t, func() bool {
		return store.Len() == 0
	}, 5*time.Second, time.Second)

	p.Close()
}

type store struct {
	mu        sync.Mutex
	unplished []telemetry.Metric
}

func newStore() *store {
	s := &store{}
	s.unplished = []telemetry.Metric{
		{
			RowID:     1,
			Timestamp: time.Now().UTC(),
			Type:      telemetry.StateHashType,
			Payload: telemetry.StateHashMetric{
				ChainID:     1337,
				BlockNumber: 1,
				Hash:        "abcdef",
			},
		},
	}
	return s
}

func (s *store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.unplished)
}

func (s *store) FetchUnpublishedMetrics(_ context.Context, _ int) ([]telemetry.Metric, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unplished, nil
}

func (s *store) MarkAsPublished(_ context.Context, _ []int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.unplished = []telemetry.Metric{}
	return nil
}
