package impl

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemSQLStoreService implements the SystemService interface using SQLStore
type SystemSQLStoreService struct {
	store sqlstore.SQLStore
}

// NewSystemSQLStoreService creates a new SystemSQLStoreService
func NewSystemSQLStoreService(store sqlstore.SQLStore) system.SystemService {
	return &SystemSQLStoreService{store}
}

// GetTableMetadata returns table's metadata fetched from SQLStore
func (s *SystemSQLStoreService) GetTableMetadata(ctx context.Context, uuid uuid.UUID) (sqlstore.TableMetadata, error) {
	table, err := s.store.GetTable(ctx, uuid)
	if err != nil {
		return sqlstore.TableMetadata{}, fmt.Errorf("error fetching the table: %s", err)
	}

	return sqlstore.TableMetadata{
		ExternalURL: fmt.Sprintf("https://tableland.com/tables/%s", uuid.String()),
		Image:       "https://hub.textile.io/thread/bafkqtqxkgt3moqxwa6rpvtuyigaoiavyewo67r3h7gsz4hov2kys7ha/buckets/bafzbeicpzsc423nuninuvrdsmrwurhv3g2xonnduq4gbhviyo5z4izwk5m/todo-list.png",
		Attributes: []sqlstore.TableMetadataAttribute{
			{
				DisplayType: "date",
				TraitType:   "created",
				Value:       table.CreatedAt.Unix(),
			},
		},
	}, nil
}
