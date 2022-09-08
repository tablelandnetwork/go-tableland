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
	"github.com/textileio/go-tableland/internal/formatter"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/tables"
)

// SQLRunner defines the run SQL interface of Tableland.
type SQLRunner interface {
	RunReadQuery(ctx context.Context, stmt string) (*tableland.UserRows, error)
}

// UserController defines the HTTP handlers for interacting with user tables.
type UserController struct {
	runner SQLRunner
}

// NewUserController creates a new UserController.
func NewUserController(runner SQLRunner) *UserController {
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

func metadataConfigToMetadata(row map[string]*tableland.ColValue, config MetadataConfig) Metadata {
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
		if x, ok := v.Value().(int); ok {
			md.Name = "#" + strconv.Itoa(x)
		} else if y, ok := v.Value().(**int); ok {
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

func userRowToMap(cols []tableland.UserColumn, row []*tableland.ColValue) map[string]*tableland.ColValue {
	m := make(map[string]*tableland.ColValue)
	for i := range cols {
		m[cols[i].Name] = row[i]
	}
	return m
}

// GetTableRow handles the GET /chain/{chainID}/tables/{id}/{key}/{value} call.
// Use format=erc721 query param to generate JSON for ERC721 metadata.
func (c *UserController) GetTableRow(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	vars := mux.Vars(r)
	format := r.URL.Query().Get("format")

	id, err := tables.NewTableID(vars["id"])
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid id format"})
		log.Ctx(r.Context()).Error().Err(err).Msg("invalid id format")
		return
	}

	chainID := vars["chainID"]
	stm := fmt.Sprintf("select prefix from registry where id=%s and chain_id=%s LIMIT 1", id.String(), chainID)
	prefixRes, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

	prefix := prefixRes.Rows[0][0].Value().(string)

	stm = fmt.Sprintf(
		"SELECT * FROM %s_%s_%s WHERE %s=%s LIMIT 1", prefix, chainID, id.String(), vars["key"], vars["value"])
	res, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

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

		row := userRowToMap(res.Columns, res.Rows[0])
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
		rw.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(rw).Encode(md)
	default:
		opts, err := formatterOptions(r)
		if err != nil {
			rw.WriteHeader(http.StatusBadRequest)
			msg := fmt.Sprintf("Invalid formatting params: %v", err)
			_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: msg})
			log.Ctx(r.Context()).Error().Err(err).Msg(msg)
			return
		}
		formatted, config, err := formatter.Format(res, opts...)
		if err != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			msg := fmt.Sprintf("Error formatting data: %v", err)
			_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: msg})
			log.Ctx(r.Context()).Error().Err(err).Msg(msg)
			return
		}
		if config.Unwrap && len(res.Rows) > 1 {
			rw.Header().Set("Content-Type", "application/jsonl+json")
		}
		rw.WriteHeader(http.StatusOK)
		_, _ = rw.Write(formatted)
	}
}

// GetTableQuery handles the GET /query?s=[statement] call.
// Use mode=columns|rows|json|lines query param to control output format.
func (c *UserController) GetTableQuery(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	stm := r.URL.Query().Get("s")
	res, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}

	opts, err := formatterOptions(r)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		msg := fmt.Sprintf("Error parsing formatting params: %v", err)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: msg})
		log.Ctx(r.Context()).Error().Err(err).Msg(msg)
		return
	}
	formatted, config, err := formatter.Format(res, opts...)
	if err != nil {
		rw.WriteHeader(http.StatusInternalServerError)
		msg := fmt.Sprintf("Error formatting data: %v", err)
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: msg})
		log.Ctx(r.Context()).Error().Err(err).Msg(msg)
		return
	}

	rw.WriteHeader(http.StatusOK)

	if config.Unwrap && len(res.Rows) > 1 {
		rw.Header().Set("Content-Type", "application/jsonl+json")
	}
	_, _ = rw.Write(formatted)
}

func (c *UserController) runReadRequest(
	ctx context.Context,
	stm string,
	rw http.ResponseWriter,
) (*tableland.UserRows, bool) {
	res, err := c.runner.RunReadQuery(ctx, stm)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Str("sql_request", stm).
			Err(err).
			Msg("executing read query")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: err.Error()})
		return nil, false
	}
	if len(res.Rows) == 0 {
		rw.WriteHeader(http.StatusNotFound)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("row not found")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Row not found"})
		return nil, false
	}
	return res, true
}

func formatterOptions(r *http.Request) ([]formatter.FormatOption, error) {
	var opts []formatter.FormatOption
	params, err := getFormatterParams(r)
	if err != nil {
		return nil, err
	}
	if params.output != nil {
		opts = append(opts, formatter.WithOutput(*params.output))
	}
	if params.extract != nil {
		opts = append(opts, formatter.WithExtract(*params.extract))
	}
	if params.unwrap != nil {
		opts = append(opts, formatter.WithUnwrap(*params.unwrap))
	}
	return opts, nil
}

type formatterParams struct {
	output  *formatter.Output
	extract *bool
	unwrap  *bool
}

func getFormatterParams(r *http.Request) (formatterParams, error) {
	c := formatterParams{}
	output := r.URL.Query().Get("output")
	extract := r.URL.Query().Get("extract")
	unwrap := r.URL.Query().Get("unwrap")
	if output != "" {
		output, ok := formatter.OutputFromString(output)
		if !ok {
			return formatterParams{}, fmt.Errorf("bad output query parameter")
		}
		c.output = &output
	}
	if extract != "" {
		extract, err := strconv.ParseBool(extract)
		if err != nil {
			return formatterParams{}, err
		}
		c.extract = &extract
	}
	if unwrap != "" {
		unwrap, err := strconv.ParseBool(unwrap)
		if err != nil {
			return formatterParams{}, err
		}
		c.unwrap = &unwrap
	}

	// Special handling for old mode param
	mode := r.URL.Query().Get("mode")
	if mode == "list" {
		v := true
		c.unwrap = &v
	} else if mode == "json" {
		v := formatter.Objects
		c.output = &v
	}

	return c, nil
}
