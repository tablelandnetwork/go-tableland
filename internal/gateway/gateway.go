package gateway

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
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

// ErrTableNotFound indicates that the table doesn't exist.
var ErrTableNotFound = errors.New("table not found")

// Gateway defines the gateway operations.
type Gateway interface {
	RunReadQuery(ctx context.Context, stmt string) (*tableland.TableData, error)
	GetTableMetadata(context.Context, tables.TableID) (sqlstore.TableMetadata, error)
	GetReceiptByTransactionHash(context.Context, common.Hash) (sqlstore.Receipt, bool, error)
}

var log = logger.With().Str("component", "gateway").Logger()

const (
	// DefaultMetadataImage is the default image for table's metadata.
	DefaultMetadataImage = "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link"

	// DefaultAnimationURL is an empty string. It means that the attribute will not appear in the JSON metadata.
	DefaultAnimationURL = ""
)

// GatewayService implements the Gateway interface using SQLStore.
type GatewayService struct {
	parser               parsing.SQLValidator
	extURLPrefix         string
	metadataRendererURI  string
	animationRendererURI string
	stores               map[tableland.ChainID]sqlstore.SystemStore
}

var _ (Gateway) = (*GatewayService)(nil)

// NewGateway creates a new gateway service.
func NewGateway(
	parser parsing.SQLValidator,
	stores map[tableland.ChainID]sqlstore.SystemStore,
	extURLPrefix string,
	metadataRendererURI string,
	animationRendererURI string,
) (Gateway, error) {
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

	return &GatewayService{
		parser:               parser,
		extURLPrefix:         extURLPrefix,
		metadataRendererURI:  metadataRendererURI,
		animationRendererURI: animationRendererURI,
		stores:               stores,
	}, nil
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (g *GatewayService) GetTableMetadata(ctx context.Context, id tables.TableID) (sqlstore.TableMetadata, error) {
	chainID, store, err := g.getStore(ctx)
	if err != nil {
		return sqlstore.TableMetadata{
			ExternalURL: fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, chainID, id),
			Image:       g.emptyMetadataImage(),
			Message:     "Chain isn't supported",
		}, nil
	}
	table, err := store.GetTable(ctx, id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Msg("error fetching the table")
			return sqlstore.TableMetadata{
				ExternalURL: fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, chainID, id),
				Image:       g.emptyMetadataImage(),
				Message:     "Failed to fetch the table",
			}, nil
		}

		return sqlstore.TableMetadata{
			ExternalURL: fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, chainID, id),
			Image:       g.emptyMetadataImage(),
			Message:     "Table not found",
		}, ErrTableNotFound
	}
	tableName := fmt.Sprintf("%s_%d_%s", table.Prefix, table.ChainID, table.ID)
	schema, err := store.GetSchemaByTableName(ctx, tableName)
	if err != nil {
		return sqlstore.TableMetadata{}, fmt.Errorf("get table schema information: %s", err)
	}

	return sqlstore.TableMetadata{
		Name:         tableName,
		ExternalURL:  fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, table.ChainID, table.ID),
		Image:        g.getMetadataImage(table.ChainID, table.ID),
		AnimationURL: g.getAnimationURL(table.ChainID, table.ID),
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
func (g *GatewayService) GetReceiptByTransactionHash(
	ctx context.Context,
	txnHash common.Hash,
) (sqlstore.Receipt, bool, error) {
	_, store, err := g.getStore(ctx)
	if err != nil {
		return sqlstore.Receipt{}, false, fmt.Errorf("chain not found: %s", err)
	}

	receipt, exists, err := store.GetReceipt(ctx, txnHash.Hex())
	if err != nil {
		return sqlstore.Receipt{}, false, fmt.Errorf("transaction receipt lookup: %s", err)
	}
	if !exists {
		return sqlstore.Receipt{}, false, nil
	}
	return sqlstore.Receipt{
		ChainID:       receipt.ChainID,
		BlockNumber:   receipt.BlockNumber,
		IndexInBlock:  receipt.IndexInBlock,
		TxnHash:       receipt.TxnHash,
		TableID:       receipt.TableID,
		Error:         receipt.Error,
		ErrorEventIdx: receipt.ErrorEventIdx,
	}, true, nil
}

// RunReadQuery allows the user to run SQL.
func (g *GatewayService) RunReadQuery(ctx context.Context, statement string) (*tableland.TableData, error) {
	readStmt, err := g.parser.ValidateReadQuery(statement)
	if err != nil {
		return nil, fmt.Errorf("validating query: %s", err)
	}

	queryResult, err := g.runSelect(ctx, readStmt)
	if err != nil {
		return nil, fmt.Errorf("running read statement: %s", err)
	}
	return queryResult, nil
}

func (g *GatewayService) runSelect(ctx context.Context, stmt parsing.ReadStmt) (*tableland.TableData, error) {
	var store sqlstore.SystemStore
	for _, store = range g.stores {
		break
	}

	queryResult, err := store.Read(ctx, stmt)
	if err != nil {
		return nil, fmt.Errorf("executing read-query: %s", err)
	}

	return queryResult, nil
}

func (g *GatewayService) getStore(ctx context.Context) (tableland.ChainID, sqlstore.SystemStore, error) {
	ctxChainID := ctx.Value(middlewares.ContextKeyChainID)
	chainID, ok := ctxChainID.(tableland.ChainID)
	if !ok {
		return 0, nil, errors.New("no chain id found in context")
	}
	store, ok := g.stores[chainID]
	if !ok {
		return 0, nil, fmt.Errorf("chain id %d isn't supported in the validator", chainID)
	}
	return chainID, store, nil
}

func (g *GatewayService) getMetadataImage(chainID tableland.ChainID, tableID tables.TableID) string {
	if g.metadataRendererURI == "" {
		return DefaultMetadataImage
	}

	return fmt.Sprintf("%s/%d/%s", g.metadataRendererURI, chainID, tableID)
}

func (g *GatewayService) getAnimationURL(chainID tableland.ChainID, tableID tables.TableID) string {
	if g.animationRendererURI == "" {
		return DefaultAnimationURL
	}

	return fmt.Sprintf("%s/?chain=%d&id=%s", g.animationRendererURI, chainID, tableID)
}

func (g *GatewayService) emptyMetadataImage() string {
	svg := `<svg width='512' height='512' xmlns='http://www.w3.org/2000/svg'><rect width='512' height='512' fill='#000'/></svg>` //nolint
	svgEncoded := base64.StdEncoding.EncodeToString([]byte(svg))
	return fmt.Sprintf("data:image/svg+xml;base64,%s", svgEncoded)
}
