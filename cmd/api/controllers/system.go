package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/errors"
)

// SystemController defines the HTTP handlers for interacting with system operations.
type SystemController struct {
	systemService system.SystemService
}

// NewSystemController creates a new SystemController.
func NewSystemController(svc system.SystemService) *SystemController {
	return &SystemController{svc}
}

// GetTable handles the GET /tables/{uuid} call.
func (c *SystemController) GetTable(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	requestUUID := vars["uuid"]
	uuid, err := uuid.Parse(requestUUID)
	if err != nil {
		rw.WriteHeader(http.StatusUnprocessableEntity)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestUUID", requestUUID).
			Msg("invalid uuid")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid uuid"})
		return
	}

	metadata, err := c.systemService.GetTableMetadata(ctx, uuid)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestUUID", requestUUID).
			Msg("failed to fetch metadata")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch metadata"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(metadata)
}

// GetTablesByController handles the GET /tables/controller/{address} call.
func (c *SystemController) GetTablesByController(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	controller := vars["address"]
	tables, err := c.systemService.GetTablesByController(ctx, controller)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestAddress", controller).
			Msg("failed to fetch tables")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch tables"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(tables)
}
