package impl

import (
	"context"
	"fmt"
	"net/url"

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
	store        sqlstore.SQLStore
}

// NewSystemSQLStoreService creates a new SystemSQLStoreService.
func NewSystemSQLStoreService(store sqlstore.SQLStore, extURLPrefix string) (system.SystemService, error) {
	if _, err := url.ParseRequestURI(extURLPrefix); err != nil {
		return nil, fmt.Errorf("invalid external url prefix: %s", err)
	}
	return &SystemSQLStoreService{
		extURLPrefix: extURLPrefix,
		store:        store,
	}, nil
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (s *SystemSQLStoreService) GetTableMetadata(
	ctx context.Context,
	id tableland.TableID) (sqlstore.TableMetadata, error) {
	table, err := s.store.GetTable(ctx, id)
	if err != nil {
		return sqlstore.TableMetadata{}, fmt.Errorf("error fetching the table: %s", err)
	}

	return sqlstore.TableMetadata{
		Name:        fmt.Sprintf("%s_%s", table.Name, table.ID),
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
	tables, err := s.store.GetTablesByController(ctx, controller)
	if err != nil {
		return []sqlstore.Table{}, fmt.Errorf("error fetching the tables: %s", err)
	}
	return tables, nil
}
