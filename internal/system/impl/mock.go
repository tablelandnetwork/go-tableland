package impl

import (
	"context"
	"errors"
	"fmt"
	"math/big"

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
	return []sqlstore.Table{
		{
			ID:         tableland.TableID(*big.NewInt(0)),
			ChainID:    tableland.ChainID(1337),
			Controller: "0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
			Prefix:     "test",
			// echo -n a:INT| shasum -a 256
			Structure: "0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee",
		},
		{
			ID:         tableland.TableID(*big.NewInt(1)),
			ChainID:    tableland.ChainID(1337),
			Controller: "0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
			Prefix:     "test2",
			// echo -n a:INT| shasum -a 256
			Structure: "0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee",
		},
	}, nil
}

// GetTablesByStructure returns all tables that share the same structure.
func (s *SystemMockService) GetTablesByStructure(ctx context.Context, structure string) ([]sqlstore.Table, error) {
	return []sqlstore.Table{
		{
			ID:         tableland.TableID(*big.NewInt(0)),
			ChainID:    tableland.ChainID(1337),
			Controller: "0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
			Prefix:     "test",
			// echo -n a:INT| shasum -a 256
			Structure: "0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee",
		},
		{
			ID:         tableland.TableID(*big.NewInt(1)),
			ChainID:    tableland.ChainID(1337),
			Controller: "0x2a891118Cf3a8FdeBb00109ea3ed4E33B82D960f",
			Prefix:     "test2",
			// echo -n a:INT| shasum -a 256
			Structure: "0605f6c6705c7c1257edb2d61d94a03ad15f1d253a5a75525c6da8cda34a99ee",
		},
	}, nil
}

// GetSchemaByTableName returns the schema of a table by its name.
func (s *SystemMockService) GetSchemaByTableName(ctx context.Context, name string) (sqlstore.TableSchema, error) {
	return sqlstore.TableSchema{
		Columns: []sqlstore.ColumnSchema{
			{
				Name:        "a",
				Type:        "int",
				Constraints: []string{"PRIMARY KEY"},
			},
			{
				Name:        "b",
				Type:        "text",
				Constraints: []string{"DEFAULT ''"},
			},
		},
		TableConstraints: []string{
			"CHECK check (a > 0)",
		},
	}, nil
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

// GetTablesByStructure returns all tables that share the same structure.
func (s *SystemMockErrService) GetTablesByStructure(ctx context.Context, structure string) ([]sqlstore.Table, error) {
	return []sqlstore.Table{}, errors.New("no table found")
}

// GetSchemaByTableName returns the schema of a table by its name.
func (s *SystemMockErrService) GetSchemaByTableName(ctx context.Context, name string) (sqlstore.TableSchema, error) {
	return sqlstore.TableSchema{}, errors.New("no table found")
}
