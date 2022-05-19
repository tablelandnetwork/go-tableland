package controllers

import (
	"encoding/json"
	"fmt"
	"net/http"

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

	req := tableland.RunReadQueryRequest{
		Statement: fmt.Sprintf("SELECT * FROM _%s WHERE %s=%s LIMIT 1",
			id.String(), vars["key"], vars["value"]),
	}
	res, err := c.runner.RunReadQuery(ctx, req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Str("sqlRequest", req.Statement).
			Err(err)

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		return
	}

	rows, ok := res.Result.(sqlstore.UserRows)
	if !ok {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("bad query result")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Bad query result"})
		return
	}
	if len(rows.Rows) == 0 {
		rw.WriteHeader(http.StatusNotFound)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("row not found")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Row not found"})
		return
	}

	row := make(map[string]interface{})
	for i, r := range rows.Columns {
		// Ignore key used for query since it's in the request path
		if r.Name != vars["key"] {
			row[r.Name] = rows.Rows[0][i]
		}
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(row)
}
