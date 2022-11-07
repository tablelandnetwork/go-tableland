package buildinfo

import "github.com/textileio/go-tableland/pkg/telemetry"

var (
	// GitCommit is set by govvv at build time.
	GitCommit = "n/a"
	// GitBranch  is set by govvv at build time.
	GitBranch = "n/a"
	// GitState  is set by govvv at build time.
	GitState = "n/a"
	// GitSummary is set by govvv at build time.
	GitSummary = "n/a"
	// BuildDate  is set by govvv at build time.
	BuildDate = "n/a"
	// Version  is set by govvv at build time.
	Version = "n/a"
)

// GetSummary returns a summary of git information.
func GetSummary() telemetry.GitSummaryMetric {
	summary := telemetry.GitSummaryMetric{
		Version:       telemetry.GitSummaryMetricV1,
		GitCommit:     GitCommit,
		GitBranch:     GitBranch,
		GitState:      GitState,
		GitSummary:    GitSummary,
		BuildDate:     BuildDate,
		BinaryVersion: Version,
	}
	return summary
}
