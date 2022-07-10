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
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid id format"})
		log.Ctx(r.Context()).Error().Err(err).Msg("invalid id format")
		return
	}

	chainID := vars["chainID"]
	stm := fmt.Sprintf("select prefix from registry where id=%s and chain_id=%s LIMIT 1", id.String(), chainID)
	res, err := c.runReadRequest(r.Context(), tableland.RunReadQueryRequest{Statement: stm, Unwrap: true, Extract: true})
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		log.Ctx(r.Context()).Error().Str("sqlRequest", stm).Err(err)
		return
	}

	var prefix string
	if err := json.Unmarshal(res.([]byte), &prefix); err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		log.Ctx(r.Context()).Error().Err(err)
		return
	}

	stm = fmt.Sprintf(
		"SELECT * FROM %s_%s_%s WHERE %s=%s LIMIT 1", prefix, chainID, id.String(), vars["key"], vars["value"],
	)
	req := tableland.RunReadQueryRequest{Statement: stm}
	if err := setReadParams(&req, r); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		log.Ctx(r.Context()).Error().Err(err)
		return
	}

	var out interface{}
	switch format {
	case "erc721":
		req.Output = tableland.Table
		res, err := c.runReadRequest(r.Context(), req)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
			log.Ctx(r.Context()).Error().Str("sqlRequest", stm).Err(err)
			return
		}
		rows := res.(*sqlstore.UserRows)
		var mdc MetadataConfig
		if err := urlquery.Unmarshal([]byte(r.URL.RawQuery), &mdc); err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid metadata config"})
			log.Ctx(r.Context()).Error().Err(err).Msg("invalid metadata config")
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
		res, err := c.runReadRequest(r.Context(), req)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
			log.Ctx(r.Context()).Error().Str("sqlRequest", stm).Err(err)
			return
		}
		out = res
	}

	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(out)
}

// GetTableQuery handles the GET /query?s=[statement] call.
// Use output=table|objects, unwrap=true|false, json_strings=true|false, extract=true|false
// query params to control output format.
func (c *UserController) GetTableQuery(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-type", "application/json")

	req := tableland.RunReadQueryRequest{}
	if err := setReadParams(&req, r); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		log.Ctx(r.Context()).Error().Err(err)
		return
	}

	res, err := c.runReadRequest(r.Context(), req)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		log.Ctx(r.Context()).Error().Str("sqlRequest", req.Statement).Err(err)
		return
	}

	rw.WriteHeader(http.StatusOK)
	encoder := json.NewEncoder(rw)

	_ = encoder.Encode(res)
}

func setReadParams(req *tableland.RunReadQueryRequest, r *http.Request) error {
	stm := r.URL.Query().Get("s")
	output := r.URL.Query().Get("output")
	extract := r.URL.Query().Get("extract")
	unwrap := r.URL.Query().Get("unwrap")
	jsonStrings := r.URL.Query().Get("json_strings")

	if stm != "" {
		req.Statement = stm
	}
	if output != "" {
		output, ok := tableland.OutputFromString(output)
		if !ok {
			return fmt.Errorf("bad output query parameter")
		}
		req.Output = output
	}
	if extract != "" {
		extract, err := strconv.ParseBool(extract)
		if err != nil {
			return err
		}
		req.Extract = extract
	}
	if unwrap != "" {
		unwrap, err := strconv.ParseBool(unwrap)
		if err != nil {
			return err
		}
		req.Unwrap = unwrap
	}
	if jsonStrings != "" {
		jsonStrings, err := strconv.ParseBool(jsonStrings)
		if err != nil {
			return err
		}
		req.JSONStrings = jsonStrings
	}
	return nil
}

func (c *UserController) runReadRequest(
	ctx context.Context,
	req tableland.RunReadQueryRequest,
) (interface{}, error) {
	res, err := c.runner.RunReadQuery(ctx, req)
	if err != nil {
		return nil, err
	}
	return res.Result, nil
}
