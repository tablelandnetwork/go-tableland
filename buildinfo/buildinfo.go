package buildinfo

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

// Summary provides a summary of git information in the binary.
type Summary struct {
	GitCommit  string
	GitBranch  string
	GitState   string
	GitSummary string
	BuildDate  string
	Version    string
}

// GetSummary returns a JSON string with a summary of git information.
func GetSummary() (Summary, error) {
	summary := Summary{
		GitCommit:  GitCommit,
		GitBranch:  GitBranch,
		GitState:   GitState,
		GitSummary: GitSummary,
		BuildDate:  BuildDate,
		Version:    Version,
	}
	return summary, nil
}
