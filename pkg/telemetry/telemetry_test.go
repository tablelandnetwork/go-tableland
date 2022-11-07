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
		err := Collect(context.Background(), gitSummary{})
		require.NoError(t, err)
		require.True(t, s.called)
	})
	t.Run("chains stack summary", func(t *testing.T) {
		s := &store{}
		metricStore = s

		require.False(t, s.called)

		err := Collect(context.Background(), chainsStackSummary(map[tableland.ChainID]int64{1: 10, 2: 20}))
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

type chainsStackSummary map[tableland.ChainID]int64

func (css chainsStackSummary) GetLastProcessedBlockNumber() map[tableland.ChainID]int64 { return css }

type gitSummary struct{}

func (gs gitSummary) GetGitCommit() string     { return "fakeGitCommit" }
func (gs gitSummary) GetGitBranch() string     { return "fakeGitBranch" }
func (gs gitSummary) GetGitState() string      { return "fakeGitState" }
func (gs gitSummary) GetGitSummary() string    { return "fakeGitSummary" }
func (gs gitSummary) GetBuildDate() string     { return "fakeGitDate" }
func (gs gitSummary) GetBinaryVersion() string { return "fakeBinaryVersion" }

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
