package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/textileio/go-tableland/buildinfo"
)

// InfraController defines the HTTP handlers for infrastructure APIs.
type InfraController struct {
}

// NewInfraController creates a new InfraController.
func NewInfraController() *InfraController {
	return &InfraController{}
}

// Version returns git information of the running binary.
func (c *InfraController) Version(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	summary := buildinfo.GetSummary()
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(summary)
}
