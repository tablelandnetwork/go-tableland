package impl

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/textileio/go-tableland/cmd/api/middlewares"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

// TablelandMesa is the main implementation of Tableland spec.
type TablelandMesa struct {
	store    sqlstore.SQLStore
	registry tableregistry.TableRegistry
	parser   parsing.SQLValidator
}

// NewTablelandMesa creates a new TablelandMesa.
func NewTablelandMesa(
	store sqlstore.SQLStore,
	registry tableregistry.TableRegistry,
	parser parsing.SQLValidator) tableland.Tableland {
	return &TablelandMesa{
		store:    store,
		registry: registry,
		parser:   parser,
	}
}

// CreateTable allows the user to create a table.
func (t *TablelandMesa) CreateTable(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	if err := t.authorize(ctx); err != nil {
		return tableland.Response{}, fmt.Errorf("checking address authorization: %s", err)
	}

	uuid, err := uuid.Parse(req.TableID)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("failed to parse uuid: %s", err)
	}

	if err := t.parser.ValidateCreateTable(req.Statement); err != nil {
		return tableland.Response{}, fmt.Errorf("query validation: %s", err)
	}
	// TODO: the two operations should be put inside a transaction
	if err := t.store.InsertTable(ctx, uuid, req.Controller, req.Type); err != nil {
		return tableland.Response{}, fmt.Errorf("inserting in table: %s", err)
	}
	if err := t.store.Write(ctx, req.Statement); err != nil {
		return tableland.Response{}, fmt.Errorf("creating user-table: %s", err)
	}
	return tableland.Response{Message: "Table created"}, nil
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
		return tableland.Response{}, fmt.Errorf("failed to parse uuid: %s", err)
	}

	queryType, err := t.parser.ValidateRunSQL(req.Statement)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("validating query: %s", err)
	}

	switch queryType {
	case parsing.ReadQuery:
		return t.runSelect(ctx, req)
	case parsing.WriteQuery:
		isOwner, err := t.isOwner(ctx, req.Controller, uuid)
		if err != nil {
			return tableland.Response{}, fmt.Errorf("failed to check authorization: %s", err)
		}

		if !isOwner {
			return tableland.Response{}, errors.New("you aren't authorized")
		}
		return t.runInsertOrUpdate(ctx, req)
	}

	return tableland.Response{}, errors.New("invalid command")
}

// Authorize is a convenience API giving the client something to call to trigger authorization.
func (t *TablelandMesa) Authorize(ctx context.Context) error {
	if err := t.authorize(ctx); err != nil {
		return fmt.Errorf("checking address authorization: %s", err)
	}
	return nil
}

func (t *TablelandMesa) runInsertOrUpdate(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	err := t.store.Write(ctx, req.Statement)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("executing write-query: %s", err)
	}
	return tableland.Response{Message: "Command executed"}, nil
}

func (t *TablelandMesa) runSelect(ctx context.Context, req tableland.Request) (tableland.Response, error) {
	data, err := t.store.Read(ctx, req.Statement)
	if err != nil {
		return tableland.Response{}, fmt.Errorf("executing read-query: %s", err)
	}

	return tableland.Response{Message: "Select executed", Data: data}, nil
}

func (t *TablelandMesa) isOwner(ctx context.Context, controller string, table uuid.UUID) (bool, error) {
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

func (t *TablelandMesa) authorize(ctx context.Context) error {
	// We want to allow access to only for addresses stored
	// in the system_auth table. The address was set in the context
	// by the JWT authentication middleware.
	address := ctx.Value(middlewares.ContextKeyAddress)
	addressString, ok := address.(string)
	if !ok || addressString == "" {
		return fmt.Errorf("no address found in context")
	}

	res, err := t.store.IsAuthorized(ctx, addressString)
	if err != nil {
		return err
	}

	if !res.IsAuthorized {
		return fmt.Errorf("address not authorized")
	}

	return nil
}
