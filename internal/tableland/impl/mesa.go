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

// TablelandMesa is the main implementation of Tableland spec.
type TablelandMesa struct {
	store    sqlstore.SQLStore
	registry tableregistry.TableRegistry
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(store sqlstore.SQLStore, registry tableregistry.TableRegistry) tableland.Tableland {
	return &TablelandMesa{
		store:    store,
		registry: registry,
	}
}

// CreateTable allows the user to create a table.
func (t *TablelandMesa) CreateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	uuid, err := uuid.Parse(req.TableID)
	if err != nil {
		return tableland.Response{Message: "Failed to parse uuid"}, err
	}

	if strings.Contains(strings.ToLower(req.Statement), "create") {
		if err := t.store.Begin(ctx); err != nil {
			return tableland.Response{Message: err.Error()}, err
		}

		if err := t.createTable(ctx, uuid, req.Controller, req.Statement); err != nil {
			if err := t.store.Rollback(ctx); err != nil {
				return tableland.Response{Message: err.Error()}, err
			}
			return tableland.Response{Message: err.Error()}, err
		}

		if err := t.store.Commit(ctx); err != nil {
			return tableland.Response{Message: err.Error()}, err
		}

		return tableland.Response{Message: "Table created"}, nil
	}

	return tableland.Response{Message: "Invalid command"}, nil
}

// UpdateTable allows the user to update a table.
func (t *TablelandMesa) UpdateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	// this is not going to be implemented
	return tableland.Response{Message: "Table updated"}, nil
}

// RunSQL allows the user to run SQL.
func (t *TablelandMesa) RunSQL(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	uuid, err := uuid.Parse(req.TableID)
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

	if strings.Contains(strings.ToLower(req.Statement), "insert") ||
		strings.Contains(strings.ToLower(req.Statement), "update") {
		return t.runInsertOrUpdate(ctx, req)
	}

	if strings.Contains(strings.ToLower(req.Statement), "select") {
		return t.runSelect(ctx, req)
	}

	return tableland.Response{Message: "Invalid command"}, nil
}

func (t *TablelandMesa) runInsertOrUpdate(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	if err := t.store.Write(ctx, req.Statement); err != nil {
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

func (t *TablelandMesa) createTable(ctx context.Context, uuid uuid.UUID, controller string, stmt string) error {
	if err := t.store.Write(ctx, stmt); err != nil {
		return err
	}

	if err := t.store.InsertTable(ctx, uuid, controller); err != nil {
		return err
	}

	return nil
}
