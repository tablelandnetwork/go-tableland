package impl

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

// TablelandMesa is the main implementation of Tableland spec
type TablelandMesa struct {
	store    sqlstore.SQLStore
	registry tableregistry.TableRegistry
}

func NewTablelandMesa(store sqlstore.SQLStore, registry tableregistry.TableRegistry) tableland.Tableland {
	return &TablelandMesa{
		store:    store,
		registry: registry,
	}
}

func (t *TablelandMesa) CreateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	if strings.Contains(strings.ToLower(req.Statement), "create") {
		err := t.store.Write(ctx, req.Statement)
		if err != nil {
			return tableland.Response{Message: err.Error()}, err
		}
		return tableland.Response{Message: "Table created"}, nil
	}

	return tableland.Response{Message: "Invalid command"}, nil
}

func (t *TablelandMesa) UpdateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	// this is not going to be implemented
	return tableland.Response{Message: "Table updated"}, nil
}

func (t *TablelandMesa) RunSQL(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	uuid, err := uuid.Parse(req.Table)
	if err != nil {
		return tableland.Response{Message: "Failed to parse uuid"}, err
	}

	isAuthorized, err := t.isAuthorized(ctx, req.Controller, uuid)
	if err != nil {
		return tableland.Response{Message: "Failed to check authorization"}, err
	}

	if !isAuthorized {
		return tableland.Response{Message: "You are not authorized"}, nil
	}

	if strings.Contains(strings.ToLower(req.Statement), "insert") || strings.Contains(strings.ToLower(req.Statement), "update") {
		return t.runInsertOrUpdate(ctx, req)
	}

	if strings.Contains(strings.ToLower(req.Statement), "select") {
		return t.runSelect(ctx, req)
	}

	return tableland.Response{Message: "Invalid command"}, nil
}

func (t *TablelandMesa) runInsertOrUpdate(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	err := t.store.Write(ctx, req.Statement)
	if err != nil {
		return tableland.Response{Message: err.Error()}, err
	}
	return tableland.Response{Message: "Command executed"}, nil
}

func (t *TablelandMesa) runSelect(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	data, err := t.store.Read(ctx, req.Statement)
	if err != nil {
		return tableland.Response{Message: err.Error()}, err
	}

	return tableland.Response{Message: "Select executed", Data: data}, nil
}

func (t *TablelandMesa) isAuthorized(ctx context.Context, controller string, table uuid.UUID) (bool, error) {
	isOwner, err := t.registry.IsOwner(ctx, common.HexToAddress(controller), t.uuidToBigInt(table))
	if err != nil {
		return false, fmt.Errorf("failed to execute contract call: %s", err)
	}
	return isOwner, nil
}

func (t *TablelandMesa) uuidToBigInt(uuid uuid.UUID) *big.Int {
	var n big.Int
	n.SetString(strings.Replace(uuid.String(), "-", "", 4), 16)
	return &n
}
