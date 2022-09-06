package storage

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"github.com/textileio/go-tableland/tests"
)

func TestCollectSqliteStore(t *testing.T) {
	t.Run("state hash", func(t *testing.T) {
		dbURI := tests.Sqlite3URI()
		s, err := New(dbURI)
		require.NoError(t, err)
		telemetry.SetMetricStore(s)

		err = telemetry.Collect(context.Background(), stateHash{})
		require.NoError(t, err)

		var timestamp, published int
		var payload string
		var typ telemetry.MetricType
		row := s.sqlDB.QueryRowContext(context.Background(), "SELECT * FROM system_metrics LIMIT 1")
		require.NoError(t, row.Scan(&timestamp, &typ, &payload, &published))

		require.Equal(t, 0, published)
		require.Equal(t, telemetry.StateHashType, typ)

		var stateHashMetric telemetry.StateHashMetric
		require.NoError(t, json.Unmarshal([]byte(payload), &stateHashMetric))
		require.Equal(t, stateHash{}.ChainID(), stateHashMetric.ChainID)
		require.Equal(t, stateHash{}.BlockNumber(), stateHashMetric.BlockNumber)
		require.Equal(t, stateHash{}.Hash(), stateHashMetric.Hash)
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
