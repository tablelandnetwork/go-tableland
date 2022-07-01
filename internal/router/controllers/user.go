package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/hetiansu5/urlquery"
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

// MetadataConfig defines columns should be mapped to erc721 metadata
// when using format=erc721 query param.
type MetadataConfig struct {
	Image            string            `query:"image"`
	ImageTransparent string            `query:"image_transparent"`
	ImageData        string            `query:"image_data"`
	ExternalURL      string            `query:"external_url"`
	Description      string            `query:"description"`
	Name             string            `query:"name"`
	Attributes       []AttributeConfig `query:"attributes"`
	BackgroundColor  string            `query:"background_color"`
	AnimationURL     string            `query:"animation_url"`
	YoutubeURL       string            `query:"youtube_url"`
}

// AttributeConfig provides formatting for a column used to drive an Attribute
// when using format=erc721 query param.
type AttributeConfig struct {
	Column      string `query:"column"`
	DisplayType string `query:"display_type"`
	TraitType   string `query:"trait_type"`
}

// Metadata represents erc721 metadata.
// Ref: https://docs.opensea.io/docs/metadata-standards
type Metadata struct {
	Image            interface{} `json:"image,omitempty"`
	ImageTransparent interface{} `json:"image_transparent,omitempty"`
	ImageData        interface{} `json:"image_data,omitempty"`
	ExternalURL      interface{} `json:"external_url,omitempty"`
	Description      interface{} `json:"description,omitempty"`
	Name             interface{} `json:"name,omitempty"`
	Attributes       []Attribute `json:"attributes,omitempty"`
	BackgroundColor  interface{} `json:"background_color,omitempty"`
	AnimationURL     interface{} `json:"animation_url,omitempty"`
	YoutubeURL       interface{} `json:"youtube_url,omitempty"`
}

// Attribute is a single entry in the "attributes" field of Metadata.
type Attribute struct {
	DisplayType string      `json:"display_type,omitempty"`
	TraitType   string      `json:"trait_type"`
	Value       interface{} `json:"value"`
}

func metadataConfigToMetadata(row map[string]interface{}, config MetadataConfig) Metadata {
	var md Metadata
	if v, ok := row[config.Image]; ok {
		md.Image = v
	}
	if v, ok := row[config.ImageTransparent]; ok {
		md.ImageTransparent = v
	}
	if v, ok := row[config.ImageData]; ok {
		md.ImageData = v
	}
	if v, ok := row[config.ExternalURL]; ok {
		md.ExternalURL = v
	}
	if v, ok := row[config.Description]; ok {
		md.Description = v
	}
	if v, ok := row[config.Name]; ok {
		// Handle the special case where the source column for name is a number
		if x, ok := v.(int); ok {
			md.Name = "#" + strconv.Itoa(x)
		} else if y, ok := v.(**int); ok {
			md.Name = "#" + strconv.Itoa(*(*y))
		} else {
			md.Name = v
		}
	}
	if v, ok := row[config.BackgroundColor]; ok {
		md.BackgroundColor = v
	}
	if v, ok := row[config.AnimationURL]; ok {
		md.AnimationURL = v
	}
	if v, ok := row[config.YoutubeURL]; ok {
		md.YoutubeURL = v
	}
	return md
}

func userRowToMap(cols []sqlstore.UserColumn, row []interface{}) map[string]interface{} {
	m := make(map[string]interface{})
	for i := range cols {
		m[cols[i].Name] = row[i]
	}
	return m
}

// GetTableRow handles the GET /chain/{chainID}/tables/{id}/{key}/{value} call.
// Use format=erc721 query param to generate JSON for ERC721 metadata.
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

	chainID := vars["chainID"]
	stm := fmt.Sprintf("select prefix from registry where id=%s and chain_id=%s LIMIT 1", id.String(), chainID)
	prefixRow, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

	prefix := **prefixRow.Rows[0][0].(**string)

	stm = fmt.Sprintf(
		"SELECT * FROM %s_%s_%s WHERE %s=%s LIMIT 1", prefix, chainID, id.String(), vars["key"], vars["value"])
	rows, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

	var out interface{}
	switch format {
	case "erc721":
		var mdc MetadataConfig
		if err := urlquery.Unmarshal([]byte(r.URL.RawQuery), &mdc); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			log.Ctx(r.Context()).
				Error().
				Err(err).
				Msg("invalid metadata config")

			_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid metadata config"})
			return
		}

		row := userRowToMap(rows.Columns, rows.Rows[0])
		md := metadataConfigToMetadata(row, mdc)
		for i, ac := range mdc.Attributes {
			if v, ok := row[mdc.Attributes[i].Column]; ok {
				md.Attributes = append(md.Attributes, Attribute{
					DisplayType: ac.DisplayType,
					TraitType:   ac.TraitType,
					Value:       v,
				})
			}
		}
		out = md
	default:
		out = rows
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(out)
}

// FormatMode is used to contract the output format of a query specified with a "mode" query param.
type FormatMode string

const (
	// ColumnsMode returns the query result with columns. This is the default mode.
	ColumnsMode FormatMode = "columns"
	// RowsMode returns the query result with rows only (no columns).
	RowsMode FormatMode = "rows"
	// JSONMode returns the query result as JSON key-value pairs.
	JSONMode FormatMode = "json"
	// CSVMode returns the query result as a comma-separated values. The first line contains the column header.
	CSVMode FormatMode = "csv"
	// ListMode returns the query result with each row on a new line. Column values are pipe-separated.
	ListMode FormatMode = "list"
)

var modes = map[string]FormatMode{
	"columns": ColumnsMode,
	"rows":    RowsMode,
	"json":    JSONMode,
	"csv":     CSVMode,
	"list":    ListMode,
}

func modeFromString(m string) (FormatMode, bool) {
	mode, ok := modes[m]
	return mode, ok
}

// GetTableQuery handles the GET /query?s=[statement] call.
// Use mode=columns|rows|json|lines query param to control output format.
func (c *UserController) GetTableQuery(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")

	stm := r.URL.Query().Get("s")
	rows, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)

	modestr := r.URL.Query().Get("mode")
	mode, ok := modeFromString(modestr)
	if !ok {
		mode = ColumnsMode
	}
	switch mode {
	case ColumnsMode:
		_ = encoder.Encode(rows)
	case RowsMode:
		_ = encoder.Encode(rows.Rows)
	case JSONMode:
		jsonrows := make([]map[string]interface{}, len(rows.Rows))
		for i, row := range rows.Rows {
			jsonrow := make(map[string]interface{}, len(row))
			for j, col := range row {
				jsonrow[rows.Columns[j].Name] = col
			}
			jsonrows[i] = jsonrow
		}
		_ = encoder.Encode(jsonrows)
	case CSVMode:
		rw.Header().Set("Content-type", "text/csv")
		for i, col := range rows.Columns {
			if i > 0 {
				_, _ = rw.Write([]byte(","))
			}
			_, _ = rw.Write([]byte(col.Name))
		}
		_, _ = rw.Write([]byte("\n"))
		for _, row := range rows.Rows {
			for j, col := range row {
				if j > 0 {
					_, _ = rw.Write([]byte(","))
				}

				b, err := json.Marshal(col)
				if err != nil {
					rw.WriteHeader(http.StatusBadRequest)
					log.Ctx(r.Context()).
						Error().
						Str("sqlRequest", stm).
						Err(err)

					_ = encoder.Encode(errors.ServiceError{Message: err.Error()})
					return
				}
				_, _ = rw.Write(b)
			}
			_, _ = rw.Write([]byte("\n"))
		}
	case ListMode:
		rw.Header().Set("Content-type", "application/jsonl+json")
		for _, row := range rows.Rows {
			for j, col := range row {
				if j > 0 {
					_, _ = rw.Write([]byte("|"))
				}

				b, err := json.Marshal(col)
				if err != nil {
					rw.WriteHeader(http.StatusBadRequest)
					log.Ctx(r.Context()).
						Error().
						Str("sqlRequest", stm).
						Err(err)

					_ = encoder.Encode(errors.ServiceError{Message: err.Error()})
					return
				}
				_, _ = rw.Write(b)
			}
			_, _ = rw.Write([]byte("\n"))
		}
	default:
		rw.WriteHeader(http.StatusBadRequest)
		err := fmt.Errorf("unrecognized mode: %s", modestr)
		log.Ctx(r.Context()).
			Error().
			Str("mode", modestr).
			Err(err)

		_ = encoder.Encode(errors.ServiceError{Message: err.Error()})
	}
}

func (c *UserController) runReadRequest(
	ctx context.Context,
	stm string,
	rw http.ResponseWriter) (*sqlstore.UserRows, bool) {
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
		return nil, false
	}

	rows, ok := res.Result.(*sqlstore.UserRows)
	if !ok {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("bad query result")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Bad query result"})
		return nil, false
	}
	if len(rows.Rows) == 0 {
		rw.WriteHeader(http.StatusNotFound)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("row not found")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Row not found"})
		return nil, false
	}
	return rows, true
}
