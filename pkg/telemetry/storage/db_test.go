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
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/telemetry"
	"github.com/textileio/go-tableland/pkg/telemetry/publisher"

	"github.com/textileio/go-tableland/tests"
)

func TestCollectAndFetchAndPublish(t *testing.T) {
	t.Parallel()

	// Notes:
	// This can't be wired per sub-tests for two reasons:
	// 1- `telemetry.SetMetricStore(...)` is a global setup at the package level, and
	// 2- `SetMetricStore(...)` has a `sync.Once` wrapping so can't be called more than once, so each sub-test can't
	//     override their value.
	//
	// This also means that sub-tests can't run in parallel.
	dbURI := tests.Sqlite3URI(t)
	s, err := New(dbURI)
	require.NoError(t, err)
	telemetry.SetMetricStore(s)

	t.Run("state hash", func(t *testing.T) {
		// collect two mocked statehash metrics
		require.NoError(t, telemetry.Collect(context.Background(), fakeStateHash))
		require.NoError(t, telemetry.Collect(context.Background(), fakeStateHash))

		metrics, err := s.FetchMetrics(context.Background(), false, 10)
		require.NoError(t, err)
		require.Len(t, metrics, 2)

		for _, metric := range metrics {
			require.Equal(t, telemetry.StateHashType, metric.Type)
			require.Equal(t, 1, metric.Version)
			require.False(t, metric.Timestamp.IsZero())

			sh := metric.Payload.(*telemetry.StateHashMetric)
			require.Equal(t, fakeStateHash.ChainID, sh.ChainID)
			require.Equal(t, fakeStateHash.BlockNumber, sh.BlockNumber)
			require.Equal(t, fakeStateHash.Hash, sh.Hash)
		}

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
			metrics, err = s.FetchMetrics(context.Background(), false, 2)
			require.NoError(t, err)
			return len(metrics) == 0
		}, 5*time.Second, time.Second)

		p.Close()
	})

	t.Run("git summary", func(t *testing.T) {
		// collect two mocked gitSummary metrics
		require.NoError(t, telemetry.Collect(context.Background(), fakeGitSummary))
		require.NoError(t, telemetry.Collect(context.Background(), fakeGitSummary))

		metrics, err := s.FetchMetrics(context.Background(), false, 10)
		require.NoError(t, err)
		require.Len(t, metrics, 2)

		for _, metric := range metrics {
			require.Equal(t, telemetry.GitSummaryType, metric.Type)
			require.Equal(t, 1, metric.Version)
			require.False(t, metric.Timestamp.IsZero())

			gv := metric.Payload.(*telemetry.GitSummaryMetric)
			require.Equal(t, fakeGitSummary.GitCommit, gv.GitCommit)
			require.Equal(t, fakeGitSummary.GitBranch, gv.GitBranch)
			require.Equal(t, fakeGitSummary.GitState, gv.GitState)
			require.Equal(t, fakeGitSummary.GitSummary, gv.GitSummary)
			require.Equal(t, fakeGitSummary.BuildDate, gv.BuildDate)
			require.Equal(t, fakeGitSummary.BinaryVersion, gv.BinaryVersion)
		}

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
			metrics, err = s.FetchMetrics(context.Background(), false, 2)
			require.NoError(t, err)
			return len(metrics) == 0
		}, 5*time.Second, time.Second)

		p.Close()
	})

	t.Run("chains stack summary", func(t *testing.T) {
		// collect two mocked chainsStackSummary metrics
		chainsStackSummaryMetrics := [2]chainsStackSummary{
			map[tableland.ChainID]int64{1: 10, 2: 20},
			map[tableland.ChainID]int64{1: 11, 2: 21},
		}
		require.NoError(t, telemetry.Collect(context.Background(), chainsStackSummaryMetrics[0]))
		require.NoError(t, telemetry.Collect(context.Background(), chainsStackSummaryMetrics[1]))

		metrics, err := s.FetchMetrics(context.Background(), false, 10)
		require.NoError(t, err)
		require.Len(t, metrics, 2)

		for i, metric := range metrics {
			require.Equal(t, telemetry.ChainStacksSummaryType, metric.Type)
			require.Equal(t, 1, metric.Version)
			require.False(t, metric.Timestamp.IsZero())

			css := metric.Payload.(*telemetry.ChainStacksMetric)
			require.Equal(t, chainsStackSummaryMetrics[i].GetLastProcessedBlockNumber(), css.LastProcessedBlockNumbers)
		}

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
			metrics, err = s.FetchMetrics(context.Background(), false, 2)
			require.NoError(t, err)
			return len(metrics) == 0
		}, 5*time.Second, time.Second)

		p.Close()
	})

	t.Run("read query", func(t *testing.T) {
		// collect two mocked read query metrics
		readQueryMetrics := [2]readQuery{{}, {}}
		require.NoError(t, telemetry.Collect(context.Background(), readQueryMetrics[0]))
		require.NoError(t, telemetry.Collect(context.Background(), readQueryMetrics[1]))

		metrics, err := s.FetchMetrics(context.Background(), false, 10)
		require.NoError(t, err)
		require.Len(t, metrics, 2)

		for i, metric := range metrics {
			require.Equal(t, telemetry.ReadQueryType, metric.Type)
			require.Equal(t, 1, metric.Version)
			require.False(t, metric.Timestamp.IsZero())

			payload := metric.Payload.(*telemetry.ReadQueryMetric)
			require.Equal(t, readQueryMetrics[i].IPAddress(), payload.IPAddress)
			require.Equal(t, readQueryMetrics[i].SQLStatement(), payload.SQLStatement)
			require.Equal(t, readQueryMetrics[i].FormatOptions(), payload.FormatOptions)
			require.Equal(t, readQueryMetrics[i].TookMilli(), payload.TookMilli)
		}

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
			metrics, err = s.FetchMetrics(context.Background(), false, 2)
			require.NoError(t, err)
			return len(metrics) == 0
		}, 5*time.Second, time.Second)

		p.Close()
	})

	t.Run("delete old metrics", func(t *testing.T) {
		// clear store
		err := s.DeletePublishedOlderThan(context.Background(), 0)

		require.NoError(t, err)
		// Store two metrics. One older than 7 days.
		err = s.StoreMetric(context.Background(), telemetry.Metric{
			Version:   1,
			Timestamp: time.Now().UTC(),
			Type:      telemetry.StateHashType,
			Payload:   fakeStateHash,
		})
		require.NoError(t, err)
		err = s.StoreMetric(context.Background(), telemetry.Metric{
			Version:   1,
			Timestamp: time.Now().UTC().Add(-24*7*time.Hour - 1), // 7 days + 1 old
			Type:      telemetry.StateHashType,
			Payload:   fakeStateHash,
		})
		require.NoError(t, err)

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
			metrics, err := s.FetchMetrics(context.Background(), true, 2)
			require.NoError(t, err)
			return len(metrics) == 1 // only one published metric is found
		}, 5*time.Second, time.Second)

		p.Close()
	})
}

type chainsStackSummary map[tableland.ChainID]int64

func (css chainsStackSummary) GetLastProcessedBlockNumber() map[tableland.ChainID]int64 { return css }

var fakeGitSummary = telemetry.GitSummaryMetric{
	Version:       telemetry.GitSummaryMetricV1,
	GitCommit:     "fakeGitCommit",
	GitBranch:     "fakeGitBranch",
	GitState:      "fakeGitState",
	GitSummary:    "fakeGitSummary",
	BuildDate:     "fakeGitDate",
	BinaryVersion: "fakeBinaryVersion",
}

var fakeStateHash = telemetry.StateHashMetric{
	Version:     telemetry.StateHashMetricV1,
	ChainID:     1,
	BlockNumber: 1,
	Hash:        "abcdefgh",
}

type readQuery struct{}

func (rq readQuery) IPAddress() string    { return "0.0.0.0" }
func (rq readQuery) SQLStatement() string { return "SELECT * FROM foo" }
func (rq readQuery) FormatOptions() telemetry.ReadQueryFormatOptions {
	return telemetry.ReadQueryFormatOptions{
		Extract: true,
		Unwrap:  false,
		Output:  "objects",
	}
}
func (rq readQuery) TookMilli() int64 { return 100 }
