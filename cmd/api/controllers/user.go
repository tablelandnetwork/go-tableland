package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// UserController defines the HTTP handlers for interacting with user tables.
type UserController struct {
	runner tableland.SQLRunner
}

// NewUserController creates a new UserController.
func NewUserController(runner tableland.SQLRunner) *UserController {
	return &UserController{runner}
}

// GetTableRow handles the GET /chain/{chainID}/tables/{id}/{key}/{value} call.
func (c *UserController) GetTableRow(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)
	format := r.URL.Query().Get("format")

	id, err := tableland.NewTableID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(r.Context()).
			Error().
			Err(err).
			Msg("invalid id format")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid id format"})
		return
	}

	stm := fmt.Sprintf("SELECT * FROM _%s WHERE %s=%s LIMIT 1", id.String(), vars["key"], vars["value"])
	rows, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

	var out interface{}
	switch format {
	case "erc721":
		md := make(map[string]interface{})
		atts := make([]map[string]interface{}, 0)
		for i, col := range rows.Columns {
			row := rows.Rows[0][i]

			// Handle the special id case (id maps to name)
			if col.Name == "id" {
				if v, ok := row.(int); ok {
					md["name"] = "#" + strconv.Itoa(v)
					continue
				} else if v, ok := row.(**int); ok {
					md["name"] = "#" + strconv.Itoa(*(*v))
					continue
				}
			}

			// Collect columns that begin with att_ as attributes
			if strings.HasPrefix(col.Name, "att_") {
				atts = append(atts, map[string]interface{}{
					"trait_type": strings.TrimPrefix(col.Name, "att_"),
					"value":      row,
				})
			} else {
				md[col.Name] = row
			}
		}
		md["attributes"] = atts
		out = md
	default:
		out = rows
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(out)
}

// GetTableQuery handles the GET /query?s=[statement] call.
func (c *UserController) GetTableQuery(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")

	rows, ok := c.runReadRequest(r.Context(), r.URL.Query().Get("s"), rw)
	if !ok {
		return
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(rows)
}

func (c *UserController) runReadRequest(
	ctx context.Context,
	stm string,
	rw http.ResponseWriter) (sqlstore.UserRows, bool) {
	req := tableland.RunReadQueryRequest{
		Statement: stm,
	}
	res, err := c.runner.RunReadQuery(ctx, req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Str("sqlRequest", req.Statement).
			Err(err)

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		return sqlstore.UserRows{}, false
	}

	rows, ok := res.Result.(sqlstore.UserRows)
	if !ok {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("bad query result")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Bad query result"})
		return sqlstore.UserRows{}, false
	}
	if len(rows.Rows) == 0 {
		rw.WriteHeader(http.StatusNotFound)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("row not found")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Row not found"})
		return sqlstore.UserRows{}, false
	}
	return rows, true
}
