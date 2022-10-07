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

// GetSummary returns a summary of git information.
func GetSummary() Summary {
	summary := Summary{
		GitCommit:  GitCommit,
		GitBranch:  GitBranch,
		GitState:   GitState,
		GitSummary: GitSummary,
		BuildDate:  BuildDate,
		Version:    Version,
	}
	return summary
}

// GetGitCommit returns the GitCommit.
func (s Summary) GetGitCommit() string { return s.GitCommit }

// GetGitBranch returns the GitBranch.
func (s Summary) GetGitBranch() string { return s.GitBranch }

// GetGitState returns the GitState.
func (s Summary) GetGitState() string { return s.GitState }

// GetGitSummary returns the GitSummary.
func (s Summary) GetGitSummary() string { return s.GitSummary }

// GetBuildDate returns the build date.
func (s Summary) GetBuildDate() string { return s.BuildDate }

// GetBinaryVersion returns the binary version.
func (s Summary) GetBinaryVersion() string { return s.Version }
