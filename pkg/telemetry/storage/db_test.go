package storage

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"github.com/textileio/go-tableland/tests"
)

func TestCollectAndFetch(t *testing.T) {
	t.Run("state hash", func(t *testing.T) {
		dbURI := tests.Sqlite3URI()
		s, err := New(dbURI)
		require.NoError(t, err)
		telemetry.SetMetricStore(s)

		err = telemetry.Collect(context.Background(), stateHash{})
		require.NoError(t, err)

		metrics, err := s.FetchUnpublishedMetrics(context.Background(), 1)
		require.NoError(t, err)

		require.Equal(t, telemetry.StateHashType, metrics[0].Type)
		require.Equal(t, stateHash{}.ChainID(), metrics[0].Payload.(*telemetry.StateHashMetric).ChainID)
		require.Equal(t, stateHash{}.BlockNumber(), metrics[0].Payload.(*telemetry.StateHashMetric).BlockNumber)
		require.Equal(t, stateHash{}.Hash(), metrics[0].Payload.(*telemetry.StateHashMetric).Hash)
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
