package impl

import (
	"context"
	"fmt"

	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

const (
	// SystemTablesPrefix is the prefix used in table names that
	// aren't owned by users, but the system.
	SystemTablesPrefix = "system_"
)

// SystemSQLStoreService implements the SystemService interface using SQLStore.
type SystemSQLStoreService struct {
	store sqlstore.SQLStore
}

// NewSystemSQLStoreService creates a new SystemSQLStoreService.
func NewSystemSQLStoreService(store sqlstore.SQLStore) system.SystemService {
	return &SystemSQLStoreService{store}
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (s *SystemSQLStoreService) GetTableMetadata(ctx context.Context, id tableland.TableID) (sqlstore.TableMetadata, error) {
	table, err := s.store.GetTable(ctx, id)
	if err != nil {
		return sqlstore.TableMetadata{}, fmt.Errorf("error fetching the table: %s", err)
	}

	return sqlstore.TableMetadata{
		ExternalURL: fmt.Sprintf("https://tableland.com/tables/%s", id),
		Image:       "https://hub.textile.io/thread/bafkqtqxkgt3moqxwa6rpvtuyigaoiavyewo67r3h7gsz4hov2kys7ha/buckets/bafzbeicpzsc423nuninuvrdsmrwurhv3g2xonnduq4gbhviyo5z4izwk5m/todo-list.png", //nolint
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

// Authorize authorizes an address in the SQLStore.
func (s *SystemSQLStoreService) Authorize(ctx context.Context, address string) error {
	if err := s.store.Authorize(ctx, address); err != nil {
		return fmt.Errorf("authorizing address: %s", err)
	}
	return nil
}

// Revoke removes an address' access in the SQLStore.
func (s *SystemSQLStoreService) Revoke(ctx context.Context, address string) error {
	if err := s.store.Revoke(ctx, address); err != nil {
		return fmt.Errorf("revoking address: %s", err)
	}
	return nil
}

// IsAuthorized checks the authorization status of an address in the SQLStore.
func (s *SystemSQLStoreService) IsAuthorized(ctx context.Context, address string) (sqlstore.IsAuthorizedResult, error) {
	res, err := s.store.IsAuthorized(ctx, address)
	if err != nil {
		return sqlstore.IsAuthorizedResult{}, fmt.Errorf("checking authorization: %s", err)
	}
	return res, nil
}

// GetAuthorizationRecord gets the authorization record for the provided address from the SQLStore.
func (s *SystemSQLStoreService) GetAuthorizationRecord(
	ctx context.Context,
	address string,
) (sqlstore.AuthorizationRecord, error) {
	record, err := s.store.GetAuthorizationRecord(ctx, address)
	if err != nil {
		return sqlstore.AuthorizationRecord{}, fmt.Errorf("getting authorization record: %s", err)
	}
	return record, nil
}

// ListAuthorized lists all authorization records in the SQLStore.
func (s *SystemSQLStoreService) ListAuthorized(ctx context.Context) ([]sqlstore.AuthorizationRecord, error) {
	records, err := s.store.ListAuthorized(ctx)
	if err != nil {
		return nil, fmt.Errorf("listing authorized addresses: %s", err)
	}
	return records, nil
}
