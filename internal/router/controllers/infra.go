package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/pkg/errors"
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
	summary, err := buildinfo.GetSummary()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(r.Context()).
			Error().
			Err(err).
			Msg("get git summary")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Can't get git summary"})
		return
	}
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(summary)
}
