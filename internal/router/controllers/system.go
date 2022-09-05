package controllers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/tables"
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

	id, err := tables.NewTableID(vars["id"])
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
			Str("request_address", controller).
			Msg("failed to fetch tables")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch tables"})
		return
	}

	// This struct is used since we don't want to return an ID field.
	// The Name will be {optional-prefix}_{chainId}_{tableId}.
	// Not doing `omitempty` in tableland.Table since
	// that feels hacky. Looks safer to define a separate type here at the handler level.
	type tableNameIDUnified struct {
		Controller string `json:"controller"`
		Name       string `json:"name"`
		Structure  string `json:"structure"`
	}
	retTables := make([]tableNameIDUnified, len(tables))
	for i, t := range tables {
		retTables[i] = tableNameIDUnified{
			Controller: t.Controller,
			Name:       t.Name(),
			Structure:  t.Structure,
		}
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(retTables)
}

// GetTablesByStructureHash handles the GET /chain/{id}/tables/structure/{hash} call.
func (c *SystemController) GetTablesByStructureHash(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	hash := vars["hash"]
	tables, err := c.systemService.GetTablesByStructure(ctx, hash)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("hash", hash).
			Msg("failed to fetch tables")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch tables"})
		return
	}

	type tableNameIDUnified struct {
		Controller string `json:"controller"`
		Name       string `json:"name"`
		Structure  string `json:"structure"`
	}
	retTables := make([]tableNameIDUnified, len(tables))
	for i, t := range tables {
		retTables[i] = tableNameIDUnified{
			Controller: t.Controller,
			Name:       t.Name(),
			Structure:  t.Structure,
		}
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(retTables)
}

// GetSchemaByTableName handles the GET /schema/{table_name} call.
func (c *SystemController) GetSchemaByTableName(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	name := vars["table_name"]
	schema, err := c.systemService.GetSchemaByTableName(ctx, name)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("table_name", name).
			Msg("failed to fetch tables")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to get schema from table"})
		return
	}

	if len(schema.Columns) == 0 {
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Warn().
			Str("name", name).
			Msg("table does not exist")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Table does not exist"})
		return
	}

	type Column struct {
		Name        string   `json:"name"`
		Type        string   `json:"type"`
		Constraints []string `json:"constraints"`
	}

	type response struct {
		Columns          []Column `json:"columns"`
		TableConstraints []string `json:"table_constraints"`
	}

	columns := make([]Column, len(schema.Columns))
	for i, col := range schema.Columns {
		columns[i] = Column{
			Name:        col.Name,
			Type:        col.Type,
			Constraints: col.Constraints,
		}
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(response{
		Columns:          columns,
		TableConstraints: schema.TableConstraints,
	})
}
