package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

// Version returns git information of the running binary.
func (c *UserController) Version(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	summary := buildinfo.GetSummary()
	rw.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(rw).Encode(apiv1.VersionInfo{
		Version:       int32(summary.Version),
		GitCommit:     summary.GitCommit,
		GitBranch:     summary.GitBranch,
		GitState:      summary.GitState,
		GitSummary:    summary.GitSummary,
		BuildDate:     summary.BuildDate,
		BinaryVersion: summary.BinaryVersion,
	})
}
