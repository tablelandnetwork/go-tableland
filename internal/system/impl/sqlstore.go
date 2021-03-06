package impl

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

const (
	// SystemTablesPrefix is the prefix used in table names that
	// aren't owned by users, but the system.
	SystemTablesPrefix = "system_"

	// RegistryTableName is a special system table (not owned by user)
	// that has information about all tables owned by users.
	RegistryTableName = "registry"
)

// SystemSQLStoreService implements the SystemService interface using SQLStore.
type SystemSQLStoreService struct {
	extURLPrefix string
	stores       map[tableland.ChainID]sqlstore.SystemStore
}

// NewSystemSQLStoreService creates a new SystemSQLStoreService.
func NewSystemSQLStoreService(
	stores map[tableland.ChainID]sqlstore.SystemStore,
	extURLPrefix string,
) (system.SystemService, error) {
	if _, err := url.ParseRequestURI(extURLPrefix); err != nil {
		return nil, fmt.Errorf("invalid external url prefix: %s", err)
	}
	return &SystemSQLStoreService{
		extURLPrefix: extURLPrefix,
		stores:       stores,
	}, nil
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (s *SystemSQLStoreService) GetTableMetadata(
	ctx context.Context,
	id tableland.TableID,
) (sqlstore.TableMetadata, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return sqlstore.TableMetadata{}, errors.New("no chain id found in context")
	}
	store, ok := s.stores[chainID]
	if !ok {
		return sqlstore.TableMetadata{}, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	table, err := store.GetTable(ctx, id)
	if err != nil {
		return sqlstore.TableMetadata{}, fmt.Errorf("error fetching the table: %s", err)
	}

	return sqlstore.TableMetadata{
		Name:        fmt.Sprintf("%s_%d_%s", table.Prefix, table.ChainID, table.ID),
		ExternalURL: fmt.Sprintf("%s/chain/%d/tables/%s", s.extURLPrefix, table.ChainID, table.ID),
		Image:       "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link", //nolint
		Attributes: []sqlstore.TableMetadataAttribute{
			{
				DisplayType: "date",
				TraitType:   "created",
				Value:       table.CreatedAt.Unix(),
			},
		},
	}, nil
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *SystemSQLStoreService) GetTablesByController(
	ctx context.Context,
	controller string,
) ([]sqlstore.Table, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return nil, errors.New("no chain id found in context")
	}
	store, ok := s.stores[chainID]
	if !ok {
		return nil, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	tables, err := store.GetTablesByController(ctx, controller)
	if err != nil {
		return nil, fmt.Errorf("error fetching the tables: %s", err)
	}
	return tables, nil
}

// GetTablesByStructure returns all tables that share the same structure.
func (s *SystemSQLStoreService) GetTablesByStructure(ctx context.Context, structure string) ([]sqlstore.Table, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return nil, errors.New("no chain id found in context")
	}
	store, ok := s.stores[chainID]
	if !ok {
		return nil, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	tables, err := store.GetTablesByStructure(ctx, structure)
	if err != nil {
		return nil, fmt.Errorf("get tables by structure: %s", err)
	}
	return tables, nil
}

// GetSchemaByTableName returns the schema of a table by its name.
func (s *SystemSQLStoreService) GetSchemaByTableName(
	ctx context.Context,
	tableName string,
) (sqlstore.TableSchema, error) {
	table, err := tableland.NewTableFromName(tableName)
	if err != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("new table from name: %s", err)
	}

	store, ok := s.stores[table.ChainID()]
	if !ok {
		return sqlstore.TableSchema{}, fmt.Errorf("chain id %d isn't supported in the validator", table.ChainID())
	}

	schema, err := store.GetSchemaByTableName(ctx, tableName)
	if err != nil {
		return sqlstore.TableSchema{}, fmt.Errorf("get schema by table name: %s", err)
	}
	return schema, nil
}
