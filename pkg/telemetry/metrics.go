package telemetry

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
	"github.com/textileio/go-tableland/internal/tableland"
)

// MetricType defines the metric type.
type MetricType int

const (
	// StateHashType is the type for the StateHashMetric.
	StateHashType MetricType = iota
	// GitSummaryType is the type for the GitSummaryMetric.
	GitSummaryType
	// ChainStacksSummaryType is the type for the ChainStacksMetric.
	ChainStacksSummaryType
	// ReadQueryType is the type for the ReadQueryMetric.
	ReadQueryType
)

// Metric defines a metric.
type Metric struct {
	RowID     int64       `json:"-"`
	Version   int         `json:"version"`
	Timestamp time.Time   `json:"timestamp"`
	Type      MetricType  `json:"type"`
	Payload   interface{} `json:"payload"`
}

// Serialize serializes the metric.
func (m Metric) Serialize() ([]byte, error) {
	b, err := json.Marshal(m.Payload)
	if err != nil {
		return []byte(nil), errors.Errorf("marshal: %s", err)
	}

	return b, nil
}

// StateHash defines how data is accessed to create a StateHashMetric.
type StateHash interface {
	ChainID() int64
	BlockNumber() int64
	Hash() string
}

// StateHashMetric defines a state hash metric.
type StateHashMetric struct {
	Version int64 `json:"version"`

	ChainID     int64  `json:"chain_id"`
	BlockNumber int64  `json:"block_number"`
	Hash        string `json:"hash"`
}

// GitSummary defines how data is accessed to create a VersionSummaryMetric.
type GitSummary interface {
	GetGitCommit() string
	GetGitBranch() string
	GetGitState() string
	GetGitSummary() string
	GetBuildDate() string
	GetBinaryVersion() string
}

// GitSummaryMetric contains Git information of the binary.
type GitSummaryMetric struct {
	Version int `json:"version"`

	GitCommit     string `json:"git_commit"`
	GitBranch     string `json:"git_branch"`
	GitState      string `json:"git_state"`
	GitSummary    string `json:"git_summary"`
	BuildDate     string `json:"build_date"`
	BinaryVersion string `json:"binary_version"`
}

// ChainStacksSummary defines how data is accessed to create a ChainStacksMetric.
type ChainStacksSummary interface {
	GetLastProcessedBlockNumber() map[tableland.ChainID]int64
}

// ChainStacksMetric contains information about each chain being synced.
type ChainStacksMetric struct {
	Version int `json:"version"`

	LastProcessedBlockNumbers map[tableland.ChainID]int64 `json:"last_processed_block_number"`
}

// ReadQuery defines how data is accessed to create a ReadQueryMetric.
type ReadQuery interface {
	IPAddress() string
	SQLStatement() string
	FormatOptions() ReadQueryFormatOptions
	TookMilli() int64
}

type ReadQueryFormatOptions struct {
	Extract bool   `json:"extract"`
	Unwrap  bool   `json:"unwrap"`
	Output  string `json:"output"`
}

// ReadQueryMetric contains information about a read query.
type ReadQueryMetric struct {
	Version int `json:"version"`

	IPAddress     string                 `json:"ip_address"`
	SQLStatement  string                 `json:"sql_statement"`
	FormatOptions ReadQueryFormatOptions `json:"format_options"`
	TookMilli     int64                  `json:"took_milli"`
}
