package telemetry

import (
	"encoding/json"
	"time"

	"github.com/pkg/errors"
)

// MetricType defines the metric type.
type MetricType int

const (
	// StateHashType is the type for the StateHashMetric.
	StateHashType MetricType = iota
	// GitSummaryType is the type for the GitSummaryMetric.
	GitSummaryType
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

// GitSummary contains Git information of the binary.
type GitSummaryMetric struct {
	Version int `json:"version"`

	GitCommit     string `json:"git_commit"`
	GitBranch     string `json:"git_branch"`
	GitState      string `json:"git_state"`
	GitSummary    string `json:"git_summary"`
	BuildDate     string `json:"build_date"`
	BinaryVersion string `json:"binary_version"`
}
