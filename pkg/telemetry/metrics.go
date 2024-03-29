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
	// NewBlockType is the type for the NewBlockMetric.
	NewBlockType
	// NewTablelandEventType is the type for the NewTablelandEventMetri.
	NewTablelandEventType
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

// StateHashMetricVersion is a type for versioning StateHash metrics.
type StateHashMetricVersion int64

// StateHashMetricV1 is the V1 version of StateHash metric.
const StateHashMetricV1 StateHashMetricVersion = iota

// StateHashMetric defines a state hash metric.
type StateHashMetric struct {
	Version StateHashMetricVersion `json:"version"`

	ChainID     int64  `json:"chain_id"`
	BlockNumber int64  `json:"block_number"`
	Hash        string `json:"hash"`
}

// GitSummaryMetricVersion is a type for versioning GitSummary metrics.
type GitSummaryMetricVersion int64

// GitSummaryMetricV1 is the V1 version of GitSummary metric.
const GitSummaryMetricV1 GitSummaryMetricVersion = iota

// GitSummaryMetric contains Git information of the binary.
type GitSummaryMetric struct {
	Version GitSummaryMetricVersion `json:"version"`

	GitCommit     string `json:"git_commit"`
	GitBranch     string `json:"git_branch"`
	GitState      string `json:"git_state"`
	GitSummary    string `json:"git_summary"`
	BuildDate     string `json:"build_date"`
	BinaryVersion string `json:"binary_version"`
}

// ChainStacksMetricVersion is a type for versioning ChainStacks metrics.
type ChainStacksMetricVersion int64

// ChainStacksMetricV1 is the V1 version of ChainStacks metric.
const ChainStacksMetricV1 ChainStacksMetricVersion = iota

// ChainStacksMetric contains information about each chain being synced.
type ChainStacksMetric struct {
	Version ChainStacksMetricVersion `json:"version"`

	LastProcessedBlockNumbers map[tableland.ChainID]int64 `json:"last_processed_block_number"`
}

// ReadQueryMetricVersion is a type for versioning ReadQuery metrics.
type ReadQueryMetricVersion int64

// ReadQueryMetricV1 is the V1 version of ReadQuery metric.
const ReadQueryMetricV1 ReadQueryMetricVersion = iota

// ReadQueryFormatOptions contains formatting configuration of a ReadQuery metric.
type ReadQueryFormatOptions struct {
	Extract bool   `json:"extract"`
	Unwrap  bool   `json:"unwrap"`
	Output  string `json:"output"`
}

// ReadQueryMetric contains information about a read query.
type ReadQueryMetric struct {
	Version ReadQueryMetricVersion `json:"version"`

	IPAddress     string                 `json:"ip_address"`
	SQLStatement  string                 `json:"sql_statement"`
	FormatOptions ReadQueryFormatOptions `json:"format_options"`
	TookMilli     int64                  `json:"took_milli"`
}

// NewBlockMetricVersion is a type for versioning NewBlock metrics.
type NewBlockMetricVersion int64

// NewBlockMetricV1 is the V1 version of NewBlock metric.
const NewBlockMetricV1 NewBlockMetricVersion = iota

// NewBlockMetric contains information about a newly detected block.
type NewBlockMetric struct {
	Version NewBlockMetricVersion `json:"version"`

	ChainID            int    `json:"chain_id"`
	BlockNumber        int64  `json:"block_number"`
	BlockTimestampUnix uint64 `json:"block_timestamp_unix"`
}

// NewTablelandEventMetricVersion is a type for versioning NewTablelandEvent metrics.
type NewTablelandEventMetricVersion int64

// NewTablelandEventMetricV1 is the V1 version of NewTablelandEvent metric.
const NewTablelandEventMetricV1 NewTablelandEventMetricVersion = iota

// NewTablelandEventMetric contains information about a newly detected Tableland event.
type NewTablelandEventMetric struct {
	Version NewTablelandEventMetricVersion `json:"version"`

	Address     string `json:"address"`
	Topics      []byte `json:"topics"`
	Data        []byte `json:"data"`
	BlockNumber uint64 `json:"block_number"`
	TxHash      string `json:"tx_hash"`
	TxIndex     uint   `json:"tx_index"`
	BlockHash   string `json:"block_hash"`
	Index       uint   `json:"index"`
	ChainID     int64  `json:"chain_id"`
	EventJSON   string `json:"event_json"`
	EventType   string `json:"event_type"`
}
