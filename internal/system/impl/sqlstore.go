package impl

import (
	"context"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/router/middlewares"
	"github.com/textileio/go-tableland/internal/system"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

var log = logger.With().Str("component", "systemsqlstore").Logger()

const (
	// SystemTablesPrefix is the prefix used in table names that
	// aren't owned by users, but the system.
	SystemTablesPrefix = "system_"

	// RegistryTableName is a special system table (not owned by user)
	// that has information about all tables owned by users.
	RegistryTableName = "registry"

	// DefaultMetadataImage is the default image for table's metadata.
	DefaultMetadataImage = "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link"

	// DefaultAnimationURL is an empty string. It means that the attribute will not appear in the JSON metadata.
	DefaultAnimationURL = ""
)

// SystemSQLStoreService implements the SystemService interface using SQLStore.
type SystemSQLStoreService struct {
	extURLPrefix         string
	metadataRendererURI  string
	animationRendererURI string
	stores               map[tableland.ChainID]sqlstore.SystemStore
}

// NewSystemSQLStoreService creates a new SystemSQLStoreService.
func NewSystemSQLStoreService(
	stores map[tableland.ChainID]sqlstore.SystemStore,
	extURLPrefix string,
	metadataRendererURI string,
	animationRendererURI string,
) (system.SystemService, error) {
	if _, err := url.ParseRequestURI(extURLPrefix); err != nil {
		return nil, fmt.Errorf("invalid external url prefix: %s", err)
	}

	metadataRendererURI = strings.TrimRight(metadataRendererURI, "/")
	if metadataRendererURI != "" {
		if _, err := url.ParseRequestURI(metadataRendererURI); err != nil {
			return nil, fmt.Errorf("metadata renderer uri could not be parsed: %s", err)
		}
	}

	animationRendererURI = strings.TrimRight(animationRendererURI, "/")
	if animationRendererURI != "" {
		if _, err := url.ParseRequestURI(animationRendererURI); err != nil {
			return nil, fmt.Errorf("animation renderer uri could not be parsed: %s", err)
		}
	}

	return &SystemSQLStoreService{
		extURLPrefix:         extURLPrefix,
		metadataRendererURI:  metadataRendererURI,
		animationRendererURI: animationRendererURI,
		stores:               stores,
	}, nil
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (s *SystemSQLStoreService) GetTableMetadata(
	ctx context.Context,
	id tables.TableID,
) (sqlstore.TableMetadata, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return sqlstore.TableMetadata{}, errors.New("no chain id found in context")
	}
	store, ok := s.stores[chainID]
	if !ok {
		return sqlstore.TableMetadata{
			ExternalURL: fmt.Sprintf("%s/chain/%d/tables/%s", s.extURLPrefix, chainID, id),
			Image:       s.emptyMetadataImage(),
			Message:     "Chain isn't supported",
		}, nil
	}
	table, err := store.GetTable(ctx, id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Msg("error fetching the table")
			return sqlstore.TableMetadata{
				ExternalURL: fmt.Sprintf("%s/chain/%d/tables/%s", s.extURLPrefix, chainID, id),
				Image:       s.emptyMetadataImage(),
				Message:     "Failed to fetch the table",
			}, nil
		}

		return sqlstore.TableMetadata{}, system.ErrTableNotFound
	}
	tableName := fmt.Sprintf("%s_%d_%s", table.Prefix, table.ChainID, table.ID)
	schema, err := store.GetSchemaByTableName(ctx, tableName)
	if err != nil {
		return sqlstore.TableMetadata{}, fmt.Errorf("get table schema information: %s", err)
	}

	return sqlstore.TableMetadata{
		Name:         tableName,
		ExternalURL:  fmt.Sprintf("%s/chain/%d/tables/%s", s.extURLPrefix, table.ChainID, table.ID),
		Image:        s.getMetadataImage(table.ChainID, table.ID),
		AnimationURL: s.getAnimationURL(table.ChainID, table.ID),
		Attributes: []sqlstore.TableMetadataAttribute{
			{
				DisplayType: "date",
				TraitType:   "created",
				Value:       table.CreatedAt.Unix(),
			},
		},
		Schema: schema,
	}, nil
}

// GetReceiptByTransactionHash returns a receipt by transaction hash.
func (s *SystemSQLStoreService) GetReceiptByTransactionHash(
	ctx context.Context,
	txnHash common.Hash,
) (sqlstore.Receipt, bool, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return sqlstore.Receipt{}, false, errors.New("no chain id found in context")
	}
	store, ok := s.stores[chainID]
	if !ok {
		return sqlstore.Receipt{}, false, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	receipt, exists, err := store.GetReceipt(ctx, txnHash.Hex())
	if err != nil {
		return sqlstore.Receipt{}, false, fmt.Errorf("transaction receipt lookup: %s", err)
	}
	if !exists {
		return sqlstore.Receipt{}, false, nil
	}
	return sqlstore.Receipt{
		ChainID:       chainID,
		BlockNumber:   receipt.BlockNumber,
		IndexInBlock:  receipt.IndexInBlock,
		TxnHash:       receipt.TxnHash,
		TableID:       receipt.TableID,
		Error:         receipt.Error,
		ErrorEventIdx: receipt.ErrorEventIdx,
	}, true, nil
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

func (s *SystemSQLStoreService) getMetadataImage(chainID tableland.ChainID, tableID tables.TableID) string {
	if s.metadataRendererURI == "" {
		return DefaultMetadataImage
	}

	return fmt.Sprintf("%s/%d/%s", s.metadataRendererURI, chainID, tableID)
}

func (s *SystemSQLStoreService) getAnimationURL(chainID tableland.ChainID, tableID tables.TableID) string {
	if s.animationRendererURI == "" {
		return DefaultAnimationURL
	}

	return fmt.Sprintf("%s/?chain=%d&id=%s", s.animationRendererURI, chainID, tableID)
}

func (s *SystemSQLStoreService) emptyMetadataImage() string {
	svg := `<svg width='512' height='512' xmlns='http://www.w3.org/2000/svg'><rect width='512' height='512' fill='#000'/></svg>` //nolint
	svgEncoded := base64.StdEncoding.EncodeToString([]byte(svg))
	return fmt.Sprintf("data:image/svg+xml;base64,%s", svgEncoded)
}
