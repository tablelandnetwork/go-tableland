package storage

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"github.com/textileio/go-tableland/pkg/telemetry/publisher"

	"github.com/textileio/go-tableland/tests"
)

func TestCollectAndFetchAndPublish(t *testing.T) {
	t.Run("state hash", func(t *testing.T) {
		dbURI := tests.Sqlite3URI()
		s, err := New(dbURI)
		require.NoError(t, err)
		telemetry.SetMetricStore(s)

		// collect two mocked statehash metrics
		require.NoError(t, telemetry.Collect(context.Background(), stateHash{}))
		require.NoError(t, telemetry.Collect(context.Background(), stateHash{}))

		metrics, err := s.FetchUnpublishedMetrics(context.Background(), 10)
		require.NoError(t, err)
		require.Len(t, metrics, 2)

		require.Equal(t, telemetry.StateHashType, metrics[0].Type)
		require.Equal(t, 1, metrics[0].Version)
		require.Equal(t, stateHash{}.ChainID(), metrics[0].Payload.(*telemetry.StateHashMetric).ChainID)
		require.Equal(t, stateHash{}.BlockNumber(), metrics[0].Payload.(*telemetry.StateHashMetric).BlockNumber)
		require.Equal(t, stateHash{}.Hash(), metrics[0].Payload.(*telemetry.StateHashMetric).Hash)

		require.Equal(t, telemetry.StateHashType, metrics[1].Type)
		require.Equal(t, 1, metrics[0].Version)
		require.Equal(t, stateHash{}.ChainID(), metrics[1].Payload.(*telemetry.StateHashMetric).ChainID)
		require.Equal(t, stateHash{}.BlockNumber(), metrics[1].Payload.(*telemetry.StateHashMetric).BlockNumber)
		require.Equal(t, stateHash{}.Hash(), metrics[1].Payload.(*telemetry.StateHashMetric).Hash)

		ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer ts.Close()

		exporter, err := publisher.NewHTTPExporter(ts.URL, "")
		require.NoError(t, err)
		nodeID := strings.Replace(uuid.NewString(), "-", "", -1)
		p := publisher.NewPublisher(s, exporter, nodeID, time.Second)
		p.Start()

		require.Eventually(t, func() bool {
			metrics, err = s.FetchUnpublishedMetrics(context.Background(), 2)
			require.NoError(t, err)
			return len(metrics) == 0
		}, 5*time.Second, time.Second)

		p.Close()
	})
}

type stateHash struct{}

func (h stateHash) ChainID() int64 {
	return 1
}

func (h stateHash) BlockNumber() int64 {
	return 1
}

func (h stateHash) Hash() string {
	return "abcdefgh"
}
