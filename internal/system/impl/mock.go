package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
)

// SystemMockService is a dummy implementation that returns a fixed value.
type SystemMockService struct{}

// NewSystemMockService creates a new SystemMockService.
func NewSystemMockService() system.SystemService {
	return &SystemMockService{}
}

// GetTableMetadata returns a fixed value for testing and demo purposes.
func (*SystemMockService) GetTableMetadata(ctx context.Context, id tableland.TableID) (sqlstore.TableMetadata, error) {
	return sqlstore.TableMetadata{
		Name:        "name-1",
		ExternalURL: fmt.Sprintf("https://tableland.network/tables/%s", id),
		Image:       "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link", //nolint
		Attributes: []sqlstore.TableMetadataAttribute{
			{
				DisplayType: "date",
				TraitType:   "created",
				Value:       1546360800,
			},
		},
	}, nil
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *SystemMockService) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	return []sqlstore.Table{}, nil
}

// SystemMockErrService is a dummy implementation that returns a fixed value.
type SystemMockErrService struct{}

// NewSystemMockErrService creates a new SystemMockErrService.
func NewSystemMockErrService() system.SystemService {
	return &SystemMockErrService{}
}

// GetTableMetadata returns a fixed value for testing and demo purposes.
func (*SystemMockErrService) GetTableMetadata(
	ctx context.Context,
	id tableland.TableID,
) (sqlstore.TableMetadata, error) {
	return sqlstore.TableMetadata{}, errors.New("table not found")
}

// GetTablesByController returns table's fetched from SQLStore by controller address.
func (s *SystemMockErrService) GetTablesByController(ctx context.Context, controller string) ([]sqlstore.Table, error) {
	return []sqlstore.Table{}, errors.New("no table found")
}
