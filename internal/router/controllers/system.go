package controllers

import (
	"encoding/json"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
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

func (c *SystemController) GetReceiptByTransactionHash(rw http.ResponseWriter, r *http.Request) {
	rw.Header().Set("Content-Type", "application/json")
	ctx := r.Context()

	paramTxnHash := mux.Vars(r)["transactionHash"]
	if _, err := common.ParseHexOrString(paramTxnHash); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		log.Ctx(ctx).Error().Err(err).Msg("invalid transaction hash")
		_ = json.NewEncoder(rw).Encode(errors.ServiceError{Message: "Invalid transaction hash"})
		return

	}
	txnHash := common.HexToHash(paramTxnHash)

	receipt, exists, err := c.systemService.GetReceiptByTransactionHash(ctx, txnHash)
	if err != nil {
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
	rw.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(rw).Encode(receiptResponse)
}

// GetTable handles the GET /chain/{chainID}/tables/{id} call.
func (c *SystemController) GetTable(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	rw.Header().Set("Content-type", "application/json")
	vars := mux.Vars(r)

	id, err := tables.NewTableID(vars["tableId"])
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
	if err == system.ErrTableNotFound {
		rw.WriteHeader(http.StatusNotFound)
		return
	}
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

	var metadataRes interface{}
	metadataRes = metadata
	// TODO(json-rpc): remove this if when dropping support. It won't be needed anymore for compatibility reasons.
	if strings.HasPrefix(r.RequestURI, "/api/v1/tables") {
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

		metadataRes = metadataV1
	}

	rw.WriteHeader(http.StatusOK)

	_ = json.NewEncoder(rw).Encode(metadataRes)
}

// TODO(json-rpc): delete when dropping support.
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

// TODO(json-rpc): delete when dropping support.
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

// TODO(json-rpc): delete when droppping support.
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

func HealthHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
}
