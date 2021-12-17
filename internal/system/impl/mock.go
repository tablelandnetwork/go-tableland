package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemMockService is a dummy implementation that returns a fixed value.
type SystemMockService struct {
}

// NewSystemMockService creates a new SystemMockService.
func NewSystemMockService() system.SystemService {
	return &SystemMockService{}
}

// GetTableMetadata returns a fixed value for testing and demo purposes.
func (*SystemMockService) GetTableMetadata(ctx context.Context, uuid uuid.UUID) (sqlstore.TableMetadata, error) {
	return sqlstore.TableMetadata{
		ExternalURL: fmt.Sprintf("https://tableland.com/tables/%s", uuid.String()),
		Image:       "https://hub.textile.io/thread/bafkqtqxkgt3moqxwa6rpvtuyigaoiavyewo67r3h7gsz4hov2kys7ha/buckets/bafzbeicpzsc423nuninuvrdsmrwurhv3g2xonnduq4gbhviyo5z4izwk5m/todo-list.png", //nolint
		Attributes: []sqlstore.TableMetadataAttribute{
			{
				DisplayType: "date",
				TraitType:   "created",
				Value:       1546360800,
			},
		},
	}, nil
}

// SystemMockErrService is a dummy implementation that returns a fixed value.
type SystemMockErrService struct {
}

// NewSystemMockErrService creates a new SystemMockErrService.
func NewSystemMockErrService() system.SystemService {
	return &SystemMockErrService{}
}

// GetTableMetadata returns a fixed value for testing and demo purposes.
func (*SystemMockErrService) GetTableMetadata(ctx context.Context, uuid uuid.UUID) (sqlstore.TableMetadata, error) {
	return sqlstore.TableMetadata{}, errors.New("table not found")
}
