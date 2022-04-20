package controllers

import (
	"bytes"
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

// GetTable handles the GET /tables/{id} call.
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

	// This struct is used since we don't want to return an ID field.
	// The Name will be {name}_t{ID}.
	// This is a requirement. Not doing `omitempty` in tableland.Table since
	// that feels hacky. Looks safer to define a separate type here at the handler level.
	type tableNameIDUnified struct {
		Controller string    `json:"controller"`
		Name       string    `json:"name"`
		Structure  string    `json:"structure"`
		CreatedAt  time.Time `json:"created_at"`
	}
	retTables := make([]tableNameIDUnified, len(tables))
	for i, t := range tables {
		retTables[i] = tableNameIDUnified{
			Controller: t.Controller,
			Name:       fmt.Sprintf("%s_%s", t.Name, t.ID),
			Structure:  t.Structure,
			CreatedAt:  t.CreatedAt,
		}
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(retTables)
}

// Authorize handles POST /authorized-addresses [address string body].
func (c *SystemController) Authorize(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")

	buf := new(bytes.Buffer)
	if _, err := buf.ReadFrom(r.Body); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("error reading request body")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: fmt.Sprintf("error reading request body: %s", err)})
		return
	}

	address := buf.String()
	err := c.systemService.Authorize(ctx, address)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestAddress", address).
			Msg("failed to authorize address")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to authorize address"})
		return
	}

	rw.WriteHeader(http.StatusOK)
}

// IsAuthorized handles GET /authorized-addresses/{address} requests.
func (c *SystemController) IsAuthorized(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")

	vars := mux.Vars(r)

	address := vars["address"]
	res, err := c.systemService.IsAuthorized(ctx, address)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestAddress", address).
			Msg("failed to check authorization")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to check authorization"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(res)
}

// Revoke handles DELETE /authorized-addresses/{address} requests.
func (c *SystemController) Revoke(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")

	vars := mux.Vars(r)

	address := vars["address"]
	err := c.systemService.Revoke(ctx, address)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestAddress", address).
			Msg("failed to revoke address")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to revoke address"})
		return
	}

	rw.WriteHeader(http.StatusOK)
}

// GetAuthorizationRecord handles GET /autorized-addresses/{address}/record requests.
func (c *SystemController) GetAuthorizationRecord(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")

	vars := mux.Vars(r)

	address := vars["address"]
	res, err := c.systemService.GetAuthorizationRecord(ctx, address)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("requestAddress", address).
			Msg("failed to get authorization record")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to get authorization record"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(res)
}

// ListAuthorized handles GET /autorized-addresses requests.
func (c *SystemController) ListAuthorized(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")

	res, err := c.systemService.ListAuthorized(ctx)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("failed to list authorization records")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to list authorization records"})
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(res)
}
