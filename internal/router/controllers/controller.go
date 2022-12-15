package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	"github.com/hetiansu5/urlquery"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/buildinfo"
	"github.com/textileio/go-tableland/internal/formatter"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/errors"
	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

// SQLRunner defines the run SQL interface of Tableland.
type SQLRunner interface {
	RunReadQuery(ctx context.Context, stmt string) (*tableland.TableData, error)
}

// UserController defines the HTTP handlers for interacting with user tables.
type UserController struct {
	runner        SQLRunner
	systemService system.SystemService
}

// NewUserController creates a new UserController.
func NewUserController(runner SQLRunner, svc system.SystemService) *UserController {
	return &UserController{
		runner:        runner,
		systemService: svc,
	}
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

func metadataConfigToMetadata(row map[string]*tableland.ColumnValue, config MetadataConfig) Metadata {
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

func userRowToMap(cols []tableland.Column, row []*tableland.ColumnValue) map[string]*tableland.ColumnValue {
	m := make(map[string]*tableland.ColumnValue)
	for i := range cols {
		m[cols[i].Name] = row[i]
	}
	return m
}

// GetTableRow handles the GET /chain/{chainID}/tables/{id}/{key}/{value} call.
// Use format=erc721 query param to generate JSON for ERC721 metadata.
// TODO(json-rpc): delete method when dropping support.
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

// Version returns git information of the running binary.
func (c *UserController) Version(rw http.ResponseWriter, _ *http.Request) {
	rw.Header().Set("Content-type", "application/json")
	summary := buildinfo.GetSummary()
	rw.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(rw).Encode(apiv1.VersionInfo{
		Version:       int32(summary.Version),
		GitCommit:     summary.GitCommit,
		GitBranch:     summary.GitBranch,
		GitState:      summary.GitState,
		GitSummary:    summary.GitSummary,
		BuildDate:     summary.BuildDate,
		BinaryVersion: summary.BinaryVersion,
	})
}

// GetReceiptByTransactionHash handles request asking for a transaction receipt.
func (c *UserController) GetReceiptByTransactionHash(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	paramTxnHash := mux.Vars(r)["transactionHash"]
	if _, err := common.ParseHexOrString(paramTxnHash); err != nil {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).Error().Err(err).Msg("invalid transaction hash")
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid transaction hash"})
		return
	}
	txnHash := common.HexToHash(paramTxnHash)

	receipt, exists, err := c.systemService.GetReceiptByTransactionHash(ctx, txnHash)
	if err != nil {
		rw.Header().Set("Content-Type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).Error().Err(err).Msg("get receipt by transaction hash")
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Get receipt by transaction hash failed"})
		return
	}
	if !exists {
		rw.WriteHeader(http.StatusNotFound)
		return
	}

	receiptResponse := apiv1.TransactionReceipt{
		TransactionHash: paramTxnHash,
		BlockNumber:     receipt.BlockNumber,
		ChainId:         int32(receipt.ChainID),
	}
	if receipt.TableID != nil {
		receiptResponse.TableId = receipt.TableID.String()
	}
	if receipt.Error != nil {
		receiptResponse.Error_ = *receipt.Error
		receiptResponse.ErrorEventIdx = int32(*receipt.ErrorEventIdx)
	}

	rw.Header().Set("Content-Type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(receiptResponse)
}

// GetTable handles the GET /chain/{chainID}/tables/{tableId} call.
func (c *UserController) GetTable(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)

	id, err := tables.NewTableID(vars["tableId"])
	if err != nil {
		rw.Header().Set("Content-type", "application/json")
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).
			Error().
			Err(err).
			Msg("invalid id format")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid id format"})
		return
	}

	isAPIV1 := strings.HasPrefix(r.RequestURI, "/api/v1/tables")

	metadata, err := c.systemService.GetTableMetadata(ctx, id)
	if err == system.ErrTableNotFound {
		if !isAPIV1 {
			rw.Header().Set("Content-type", "application/json")
			rw.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(rw).Encode(metadata)
			return
		}
		rw.WriteHeader(http.StatusNotFound)
		return
	}
	if err != nil {
		rw.Header().Set("Content-type", "application/json")
		rw.WriteHeader(http.StatusInternalServerError)
		log.Ctx(ctx).
			Error().
			Err(err).
			Str("id", id.String()).
			Msg("failed to fetch metadata")

		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Failed to fetch metadata"})
		return
	}

	metadataV1 := apiv1.Table{
		Name:         metadata.Name,
		ExternalUrl:  metadata.ExternalURL,
		AnimationUrl: metadata.AnimationURL,
		Image:        metadata.Image,
		Attributes:   make([]apiv1.TableAttributes, len(metadata.Attributes)),
		Schema: &apiv1.Schema{
			Columns:          make([]apiv1.Column, len(metadata.Schema.Columns)),
			TableConstraints: make([]string, len(metadata.Schema.TableConstraints)),
		},
	}
	for i, attr := range metadata.Attributes {
		metadataV1.Attributes[i] = apiv1.TableAttributes{
			DisplayType: attr.DisplayType,
			TraitType:   attr.TraitType,
			Value:       attr.Value,
		}
	}
	for i, schemaColumn := range metadata.Schema.Columns {
		metadataV1.Schema.Columns[i] = apiv1.Column{
			Name:        schemaColumn.Name,
			Type_:       schemaColumn.Type,
			Constraints: make([]string, len(schemaColumn.Constraints)),
		}
		copy(metadataV1.Schema.Columns[i].Constraints, schemaColumn.Constraints)
	}
	copy(metadataV1.Schema.TableConstraints, metadata.Schema.TableConstraints)

	rw.Header().Set("Content-type", "application/json")
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(metadataV1)
}

// GetTablesByController handles the GET /chain/{chainID}/tables/controller/{address} call.
// TODO(json-rpc): delete when dropping support.
func (c *UserController) GetTablesByController(rw http.ResponseWriter, r *http.Request) {
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
// TODO(json-rpc): delete when dropping support.
func (c *UserController) GetTablesByStructureHash(rw http.ResponseWriter, r *http.Request) {
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
// TODO(json-rpc): delete when droppping support.
func (c *UserController) GetSchemaByTableName(rw http.ResponseWriter, r *http.Request) {
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

// HealthHandler serves health check requests.
func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}

// GetTableQuery handles the GET /query?s=[statement] call.
// Use mode=columns|rows|json|lines query param to control output format.
func (c *UserController) GetTableQuery(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")

	stm := r.URL.Query().Get("s") // TODO(json-rpc): remove query parameter "s" when dropping support.
	if stm == "" {
		stm = r.URL.Query().Get("statement")
	}

	start := time.Now()
	res, ok := c.runReadRequest(r.Context(), stm, rw)
	if !ok {
		return
	}
	took := time.Since(start)

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

	CollectReadQueryMetric(r.Context(), stm, config, took)

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
) (*tableland.TableData, bool) {
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
	output := r.URL.Query().Get("output") // TODO(json-rpc): drop "output" when dropping support.
	if output == "" {
		output = r.URL.Query().Get("format")
	}

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
		c.extract = &v
	} else if mode == "json" {
		v := formatter.Objects
		c.output = &v
	}

	return c, nil
}

// CollectReadQueryMetric collects read query metric.
// It is used for JSON-RPC service. When that is deleted we can make this private.
func CollectReadQueryMetric(ctx context.Context, statement string, config formatter.FormatConfig, took time.Duration) {
	value := ctx.Value(middlewares.ContextIPAddress)
	ipAddress, ok := value.(string)
	if ok && ipAddress != "" {
		formatOptions := telemetry.ReadQueryFormatOptions{
			Extract: config.Extract,
			Unwrap:  config.Unwrap,
			Output:  string(config.Output),
		}

		metric := telemetry.ReadQueryMetric{
			Version:       telemetry.ReadQueryMetricV1,
			IPAddress:     ipAddress,
			SQLStatement:  statement,
			FormatOptions: formatOptions,
			TookMilli:     took.Milliseconds(),
		}
		if err := telemetry.Collect(ctx, metric); err != nil {
			log.Warn().Err(err).Msg("failed to collect metric")
		}
	} else {
		log.Warn().Str("sql_statement", statement).Msg("ip address not detected. metric not sent")
	}
}
