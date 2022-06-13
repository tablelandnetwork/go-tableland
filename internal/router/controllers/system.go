package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
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

// GetTable handles the GET /chain/{chainID}/tables/{id} call.
func (c *SystemController) GetTable(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	id, err := tableland.NewTableID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("invalid id format")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid id format"})
		return
	}

	metadata, err := c.systemService.GetTableMetadata(ctx, id)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("id", id.String()).
			Msg("failed to fetch metadata")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch metadata"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(metadata)
}

// TableNameIDUnified is used since we don't want to return an ID field.
// The Name will be {optional-prefix}_{chainId}_{tableId}.
// Not doing `omitempty` in tableland.Table since
// that feels hacky. Looks safer to define a separate type here at the handler level.
type TableNameIDUnified struct {
	Controller string    `json:"controller"`
	Name       string    `json:"name"`
	Structure  string    `json:"structure"`
	CreatedAt  time.Time `json:"created_at"`
}

// GetTablesByController handles the GET /chain/{chainID}/tables/controller/{address} call.
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
	retTables := make([]TableNameIDUnified, len(tables))
	for i, t := range tables {
		retTables[i] = TableNameIDUnified{
			Controller: t.Controller,
			Name:       fmt.Sprintf("%s_%d_%s", t.Prefix, t.ChainID, t.ID),
			Structure:  t.Structure,
			CreatedAt:  t.CreatedAt,
		}
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(retTables)
}
