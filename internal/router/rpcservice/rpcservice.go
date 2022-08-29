package rpcservice

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
)

// RPCService provides the JSON RPC API.
type RPCService struct {
	tbl tableland.Tableland
}

// NewRPCService creates a new RPCService.
func NewRPCService(tbl tableland.Tableland) RPCService {
	return RPCService{
		tbl: tbl,
	}
}

// ValidateCreateTable allows to validate a CREATE TABLE statement and also return the structure hash of it.
// This RPC method is stateless.
func (rs *RPCService) ValidateCreateTable(
	ctx context.Context,
	req tableland.ValidateCreateTableRequest,
) (tableland.ValidateCreateTableResponse, error) {
	return rs.tbl.ValidateCreateTable(ctx, req)
}

// ValidateWriteQuery allows the user to validate a write query.
func (rs *RPCService) ValidateWriteQuery(
	ctx context.Context,
	req tableland.ValidateWriteQueryRequest,
) (tableland.ValidateWriteQueryResponse, error) {
	return rs.tbl.ValidateWriteQuery(ctx, req)
}

// RelayWriteQuery allows the user to rely on the validator wrapping the query in a chain transaction.
func (rs *RPCService) RelayWriteQuery(
	ctx context.Context,
	req tableland.RelayWriteQueryRequest,
) (tableland.RelayWriteQueryResponse, error) {
	return rs.tbl.RelayWriteQuery(ctx, req)
}

// RunReadQuery allows the user to run SQL.
func (rs *RPCService) RunReadQuery(
	ctx context.Context,
	req tableland.RunReadQueryRequest,
) (tableland.RunReadQueryResponse, error) {
	return rs.tbl.RunReadQuery(ctx, req)
}

// GetReceipt returns the receipt of a processed event by txn hash.
func (rs *RPCService) GetReceipt(
	ctx context.Context,
	req tableland.GetReceiptRequest,
) (tableland.GetReceiptResponse, error) {
	return rs.tbl.GetReceipt(ctx, req)
}

// SetController allows users to the controller for a token id.
func (rs *RPCService) SetController(
	ctx context.Context,
	req tableland.SetControllerRequest,
) (tableland.SetControllerResponse, error) {
	return rs.tbl.SetController(ctx, req)
}
