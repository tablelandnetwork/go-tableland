package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
)

func TestCollectWithoutStore(t *testing.T) {
	metricStore = nil
	require.NoError(t, Collect(context.Background(), fakeStateHash))
}

func TestCollectMockedtStore(t *testing.T) {
	t.Run("state hash", func(t *testing.T) {
		s := &store{}
		metricStore = s

		require.False(t, s.called)
		err := Collect(context.Background(), fakeStateHash)
		require.NoError(t, err)
		require.True(t, s.called)
	})
	t.Run("git summary", func(t *testing.T) {
		s := &store{}
		metricStore = s

		require.False(t, s.called)
		err := Collect(context.Background(), fakeGitSummary)
		require.NoError(t, err)
		require.True(t, s.called)
	})
	t.Run("chains stack summary", func(t *testing.T) {
		s := &store{}
		metricStore = s

		require.False(t, s.called)

		metric := ChainStacksMetric{
			Version:                   ChainStacksMetricV1,
			LastProcessedBlockNumbers: map[tableland.ChainID]int64{1: 10, 2: 20},
		}
		err := Collect(context.Background(), metric)
		require.NoError(t, err)
		require.True(t, s.called)
	})
	t.Run("new block", func(t *testing.T) {
		s := &store{}
		metricStore = s

		require.False(t, s.called)

		metric := NewBlockMetric{
			Version:            NewBlockMetricV1,
			ChainID:            10,
			BlockNumber:        11,
			BlockTimestampUnix: 12,
		}
		err := Collect(context.Background(), metric)
		require.NoError(t, err)
		require.True(t, s.called)
	})
	t.Run("new tableland event", func(t *testing.T) {
		s := &store{}
		metricStore = s

		require.False(t, s.called)

		metric := NewTablelandEventMetric{
			Version:     NewTablelandEventMetricV1,
			Address:     "addr",
			Topics:      []byte("topics"),
			Data:        []byte("data"),
			BlockNumber: 1,
			TxHash:      "txhash",
			TxIndex:     2,
			BlockHash:   "blockhash",
			Index:       3,
			ChainID:     4,
			EventJSON:   "eventjson",
			EventType:   "eventtype",
		}
		err := Collect(context.Background(), metric)
		require.NoError(t, err)
		require.True(t, s.called)
	})
}

func TestCollectUnknownMetric(t *testing.T) {
	s := &store{}
	metricStore = s

	err := Collect(context.Background(), struct{}{})
	require.Error(t, err)
	require.ErrorContains(t, err, "unknown metric")
}

var fakeGitSummary = GitSummaryMetric{
	Version:       GitSummaryMetricV1,
	GitCommit:     "fakeGitCommit",
	GitBranch:     "fakeGitBranch",
	GitState:      "fakeGitState",
	GitSummary:    "fakeGitSummary",
	BuildDate:     "fakeGitDate",
	BinaryVersion: "fakeBinaryVersion",
}

var fakeStateHash = StateHashMetric{
	Version:     StateHashMetricV1,
	ChainID:     1,
	BlockNumber: 1,
	Hash:        "abcdefgh",
}

type store struct {
	called bool
}

func (db *store) StoreMetric(_ context.Context, _ Metric) error {
	db.called = true
	return nil
}

func (db *store) Close() error {
	return nil
}
