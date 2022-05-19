package impl

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	"github.com/textileio/go-tableland/cmd/api/middlewares"
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
	stores       map[tableland.ChainID]sqlstore.SQLStore
}

// NewSystemSQLStoreService creates a new SystemSQLStoreService.
func NewSystemSQLStoreService(
	stores map[tableland.ChainID]sqlstore.SQLStore,
	extURLPrefix string) (system.SystemService, error) {
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
	id tableland.TableID) (sqlstore.TableMetadata, error) {
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
		Name:        fmt.Sprintf("%s_%s", table.Prefix, table.ID),
		ExternalURL: fmt.Sprintf("%s/%s", s.extURLPrefix, id),
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
	controller string) ([]sqlstore.Table, error) {
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
