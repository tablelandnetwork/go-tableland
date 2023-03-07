package legacy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/formatter"
	"github.com/textileio/go-tableland/internal/router/controllers"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/tables"
)

// RelayWriteQueryRequest is a user RelayWriteQuery request.
type RelayWriteQueryRequest struct {
	Statement string `json:"statement"`
}

// RelayWriteQueryResponse is a RelayWriteQuery response.
type RelayWriteQueryResponse struct {
	Transaction struct {
		Hash string `json:"hash"`
	} `json:"tx"`
}

// RunReadQueryRequest is a user RunReadQuery request.
type RunReadQueryRequest struct {
	Statement string  `json:"statement"`
	Output    *string `json:"output"`
	Unwrap    *bool   `json:"unwrap"`
	Extract   *bool   `json:"extract"`
}

// FormatOpts extracts formatter options from a request.
func (rrqr *RunReadQueryRequest) FormatOpts() ([]formatter.FormatOption, error) {
	var opts []formatter.FormatOption
	if rrqr.Output != nil {
		output, ok := formatter.OutputFromString(*rrqr.Output)
		if !ok {
			return nil, fmt.Errorf("%s is not a valid output", *rrqr.Output)
		}
		opts = append(opts, formatter.WithOutput(output))
	}
	if rrqr.Extract != nil {
		opts = append(opts, formatter.WithExtract(*rrqr.Extract))
	}
	if rrqr.Unwrap != nil {
		opts = append(opts, formatter.WithUnwrap(*rrqr.Unwrap))
	}
	return opts, nil
}

// RunReadQueryResponse is a RunReadQuery response.
type RunReadQueryResponse struct {
	Result interface{} `json:"data"`
}

// GetReceiptRequest is a GetTxnReceipt request.
type GetReceiptRequest struct {
	TxnHash string `json:"txn_hash"`
}

// TxnReceipt is a Tableland event processing receipt.
type TxnReceipt struct {
	ChainID     int64  `json:"chain_id"`
	TxnHash     string `json:"txn_hash"`
	BlockNumber int64  `json:"block_number"`

	TableID       *string `json:"table_id,omitempty"`
	Error         string  `json:"error"`
	ErrorEventIdx int     `json:"error_event_idx"`
}

// GetReceiptResponse is a GetTxnReceipt response.
type GetReceiptResponse struct {
	Ok      bool        `json:"ok"`
	Receipt *TxnReceipt `json:"receipt,omitempty"`
}

// ValidateCreateTableRequest is a ValidateCreateTable request.
type ValidateCreateTableRequest struct {
	CreateStatement string `json:"create_statement"`
}

// ValidateCreateTableResponse is a ValidateCreateTable response.
type ValidateCreateTableResponse struct {
	StructureHash string `json:"structure_hash"`
}

// ValidateWriteQueryRequest is a ValidateWriteQuery request.
type ValidateWriteQueryRequest struct {
	Statement string `json:"statement"`
}

// ValidateWriteQueryResponse is a ValidateWriteQuery response.
type ValidateWriteQueryResponse struct {
	TableID string `json:"table_id"`
}

// SetControllerRequest is a user SetController request.
type SetControllerRequest struct {
	Controller string `json:"controller"`
	TokenID    string `json:"token_id"`
}

// SetControllerResponse is a RunSQL response.
type SetControllerResponse struct {
	Transaction struct {
		Hash string `json:"hash"`
	} `json:"tx"`
}

// RPCService provides the JSON RPC API.
type RPCService struct {
	tbl tableland.Tableland
}

// NewRPCService creates a new RPCService.
func NewRPCService(tbl tableland.Tableland) *RPCService {
	return &RPCService{
		tbl: tbl,
	}
}

// ValidateCreateTable allows to validate a CREATE TABLE statement and also return the structure hash of it.
// This RPC method is stateless.
func (rs *RPCService) ValidateCreateTable(
	ctx context.Context,
	req ValidateCreateTableRequest,
) (ValidateCreateTableResponse, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return ValidateCreateTableResponse{}, errors.New("no chain id found in context")
	}
	hash, err := rs.tbl.ValidateCreateTable(ctx, chainID, req.CreateStatement)
	if err != nil {
		return ValidateCreateTableResponse{}, fmt.Errorf("calling ValidateCreateTable %v", err)
	}
	return ValidateCreateTableResponse{StructureHash: hash}, nil
}

// ValidateWriteQuery allows the user to validate a write query.
func (rs *RPCService) ValidateWriteQuery(
	ctx context.Context,
	req ValidateWriteQueryRequest,
) (ValidateWriteQueryResponse, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return ValidateWriteQueryResponse{}, errors.New("no chain id found in context")
	}
	tableID, err := rs.tbl.ValidateWriteQuery(ctx, chainID, req.Statement)
	if err != nil {
		return ValidateWriteQueryResponse{}, fmt.Errorf("calling ValidateWriteQuery: %v", err)
	}
	return ValidateWriteQueryResponse{TableID: tableID.String()}, nil
}

// RelayWriteQuery allows the user to rely on the validator wrapping the query in a chain transaction.
func (rs *RPCService) RelayWriteQuery(
	ctx context.Context,
	req RelayWriteQueryRequest,
) (RelayWriteQueryResponse, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return RelayWriteQueryResponse{}, errors.New("no chain id found in context")
	}
	ctxCaller := ctx.Value(middlewares.ContextKeyAddress)
	caller, ok := ctxCaller.(string)
	if !ok || caller == "" {
		return RelayWriteQueryResponse{}, errors.New("no controller address found in context")
	}
	txn, err := rs.tbl.RelayWriteQuery(ctx, chainID, common.HexToAddress(caller), req.Statement)
	if err != nil {
		return RelayWriteQueryResponse{}, fmt.Errorf("calling RelayWriteQuery: %v", err)
	}
	ret := RelayWriteQueryResponse{}
	ret.Transaction.Hash = txn.Hash().Hex()
	return ret, nil
}

// RunReadQuery allows the user to run SQL.
func (rs *RPCService) RunReadQuery(
	ctx context.Context,
	req RunReadQueryRequest,
) (RunReadQueryResponse, error) {
	start := time.Now()
	res, err := rs.tbl.RunReadQuery(ctx, req.Statement)
	if err != nil {
		return RunReadQueryResponse{}, fmt.Errorf("calling RunReadQuery: %v", err)
	}
	took := time.Since(start)

	opts, err := req.FormatOpts()
	if err != nil {
		return RunReadQueryResponse{}, fmt.Errorf("getting format opts from request: %v", err)
	}

	formatted, config, err := formatter.Format(res, opts...)
	if err != nil {
		return RunReadQueryResponse{}, fmt.Errorf("formatting result: %v", err)
	}

	if config.Unwrap && len(res.Rows) > 1 {
		return RunReadQueryResponse{}, errors.New("unwrapped results with more than one row aren't supported in JSON RPC API")
	}

	controllers.CollectReadQueryMetric(ctx, req.Statement, config, took)

	return RunReadQueryResponse{Result: json.RawMessage(formatted)}, nil
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (rs *RPCService) GetReceipt(
	ctx context.Context,
	req GetReceiptRequest,
) (GetReceiptResponse, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return GetReceiptResponse{}, errors.New("no chain id found in context")
	}
	ok, receipt, err := rs.tbl.GetReceipt(ctx, chainID, req.TxnHash)
	if err != nil {
		if strings.Contains(err.Error(), "database table is locked") ||
			strings.Contains(err.Error(), "database schema is locked") {
			ret := GetReceiptResponse{Ok: ok}
			return ret, nil
		}
		return GetReceiptResponse{}, fmt.Errorf("calling GetReceipt: %v", err)
	}
	ret := GetReceiptResponse{Ok: ok}
	if ok {
		ret.Receipt = &TxnReceipt{
			ChainID:       int64(receipt.ChainID),
			TxnHash:       receipt.TxnHash,
			BlockNumber:   receipt.BlockNumber,
			TableID:       receipt.TableID,
			Error:         receipt.Error,
			ErrorEventIdx: receipt.ErrorEventIdx,
		}
	}
	return ret, nil
}

// SetController allows users to the controller for a token id.
func (rs *RPCService) SetController(
	ctx context.Context,
	req SetControllerRequest,
) (SetControllerResponse, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return SetControllerResponse{}, errors.New("no chain id found in context")
	}
	ctxCaller := ctx.Value(middlewares.ContextKeyAddress)
	caller, ok := ctxCaller.(string)
	if !ok || caller == "" {
		return SetControllerResponse{}, errors.New("no caller address found in context")
	}
	tableID, err := tables.NewTableID(req.TokenID)
	if err != nil {
		return SetControllerResponse{}, fmt.Errorf("parsing token ID: %v", err)
	}
	txn, err := rs.tbl.SetController(
		ctx, chainID,
		common.HexToAddress(caller),
		common.HexToAddress(req.Controller),
		tableID,
	)
	if err != nil {
		return SetControllerResponse{}, fmt.Errorf("calling SetController: %v", err)
	}
	ret := SetControllerResponse{}
	ret.Transaction.Hash = txn.Hash().Hex()
	return ret, nil
}
