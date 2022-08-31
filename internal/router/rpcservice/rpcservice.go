package rpcservice

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
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
	Statement string `json:"statement"`
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
	hash, err := rs.tbl.ValidateCreateTable(ctx, req.CreateStatement)
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
	tableID, err := rs.tbl.ValidateWriteQuery(ctx, req.Statement)
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
	txn, err := rs.tbl.RelayWriteQuery(ctx, req.Statement)
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
	res, err := rs.tbl.RunReadQuery(ctx, req.Statement)
	if err != nil {
		return RunReadQueryResponse{}, fmt.Errorf("calling RunReadQuery: %v", err)
	}
	return RunReadQueryResponse{Result: res}, nil
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (rs *RPCService) GetReceipt(
	ctx context.Context,
	req GetReceiptRequest,
) (GetReceiptResponse, error) {
	ok, receipt, err := rs.tbl.GetReceipt(ctx, req.TxnHash)
	if err != nil {
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
	tableID, err := tables.NewTableID(req.TokenID)
	if err != nil {
		return SetControllerResponse{}, fmt.Errorf("parsing token ID: %v", err)
	}
	txn, err := rs.tbl.SetController(ctx, common.HexToAddress(req.Controller), tableID)
	if err != nil {
		return SetControllerResponse{}, fmt.Errorf("calling SetController: %v", err)
	}
	ret := SetControllerResponse{}
	ret.Transaction.Hash = txn.Hash().Hex()
	return ret, nil
}
