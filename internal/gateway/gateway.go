package gateway

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables"
)

// ErrTableNotFound indicates that the table doesn't exist.
var ErrTableNotFound = errors.New("table not found")

var log = logger.With().Str("component", "gateway").Logger()

const (
	// DefaultMetadataImage is the default image for table's metadata.
	DefaultMetadataImage = "https://bafkreifhuhrjhzbj4onqgbrmhpysk2mop2jimvdvfut6taiyzt2yqzt43a.ipfs.dweb.link"

	// DefaultAnimationURL is an empty string. It means that the attribute will not appear in the JSON metadata.
	DefaultAnimationURL = ""
)

// Gateway defines the gateway operations.
type Gateway interface {
	RunReadQuery(ctx context.Context, stmt string) (*TableData, error)
	GetTableMetadata(context.Context, tableland.ChainID, tables.TableID) (TableMetadata, error)
	GetReceiptByTransactionHash(context.Context, tableland.ChainID, common.Hash) (Receipt, bool, error)
}

// GatewayStore is the storage layer of the Gateway.
type GatewayStore interface {
	Read(context.Context, parsing.ReadStmt) (*TableData, error)
	GetTable(context.Context, tableland.ChainID, tables.TableID) (Table, error)
	GetSchemaByTableName(context.Context, string) (TableSchema, error)
	GetReceipt(context.Context, tableland.ChainID, string) (Receipt, bool, error)
}

// GatewayService implements the Gateway interface using SQLStore.
type GatewayService struct {
	parser               parsing.SQLValidator
	extURLPrefix         string
	metadataRendererURI  string
	animationRendererURI string
	store                GatewayStore
}

var _ (Gateway) = (*GatewayService)(nil)

// NewGateway creates a new gateway service.
func NewGateway(
	parser parsing.SQLValidator,
	store GatewayStore,
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
		store:                store,
	}, nil
}

// GetTableMetadata returns table's metadata fetched from SQLStore.
func (g *GatewayService) GetTableMetadata(
	ctx context.Context, chainID tableland.ChainID, id tables.TableID,
) (TableMetadata, error) {
	table, err := g.store.GetTable(ctx, chainID, id)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			log.Error().Err(err).Msg("error fetching the table")
			return TableMetadata{
				ExternalURL: fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, chainID, id),
				Image:       g.emptyMetadataImage(),
				Message:     "Failed to fetch the table",
			}, nil
		}

		return TableMetadata{
			ExternalURL: fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, chainID, id),
			Image:       g.emptyMetadataImage(),
			Message:     "Table not found",
		}, ErrTableNotFound
	}
	tableName := fmt.Sprintf("%s_%d_%s", table.Prefix, table.ChainID, table.ID)
	schema, err := g.store.GetSchemaByTableName(ctx, tableName)
	if err != nil {
		return TableMetadata{}, fmt.Errorf("get table schema information: %s", err)
	}

	return TableMetadata{
		Name:         tableName,
		ExternalURL:  fmt.Sprintf("%s/api/v1/tables/%d/%s", g.extURLPrefix, table.ChainID, table.ID),
		Image:        g.getMetadataImage(table.ChainID, table.ID),
		AnimationURL: g.getAnimationURL(table.ChainID, table.ID),
		Attributes: []TableMetadataAttribute{
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
	ctx context.Context, chainID tableland.ChainID, txnHash common.Hash,
) (Receipt, bool, error) {
	receipt, exists, err := g.store.GetReceipt(ctx, chainID, txnHash.Hex())
	if err != nil {
		return Receipt{}, false, fmt.Errorf("transaction receipt lookup: %s", err)
	}
	if !exists {
		return Receipt{}, false, nil
	}
	return Receipt{
		ChainID:       receipt.ChainID,
		BlockNumber:   receipt.BlockNumber,
		IndexInBlock:  receipt.IndexInBlock,
		TxnHash:       receipt.TxnHash,
		TableIDs:      receipt.TableIDs,
		Error:         receipt.Error,
		ErrorEventIdx: receipt.ErrorEventIdx,

		// Deprecated
		TableID: receipt.TableID,
	}, true, nil
}

// RunReadQuery allows the user to run SQL.
func (g *GatewayService) RunReadQuery(ctx context.Context, statement string) (*TableData, error) {
	readStmt, err := g.parser.ValidateReadQuery(statement)
	if err != nil {
		return nil, fmt.Errorf("validating read query: %s", err)
	}

	queryResult, err := g.store.Read(ctx, readStmt)
	if err != nil {
		return nil, fmt.Errorf("running read statement: %s", err)
	}
	return queryResult, nil
}

func (g *GatewayService) getMetadataImage(chainID tableland.ChainID, tableID tables.TableID) string {
	if g.metadataRendererURI == "" {
		return DefaultMetadataImage
	}

	return fmt.Sprintf("%s/%d/%s.svg", g.metadataRendererURI, chainID, tableID)
}

func (g *GatewayService) getAnimationURL(chainID tableland.ChainID, tableID tables.TableID) string {
	if g.animationRendererURI == "" {
		return DefaultAnimationURL
	}

	return fmt.Sprintf("%s/%d/%s.html", g.animationRendererURI, chainID, tableID)
}

func (g *GatewayService) emptyMetadataImage() string {
	svg := `<svg width='512' height='512' xmlns='http://www.w3.org/2000/svg'><rect width='512' height='512' fill='#000'/></svg>` //nolint
	svgEncoded := base64.StdEncoding.EncodeToString([]byte(svg))
	return fmt.Sprintf("data:image/svg+xml;base64,%s", svgEncoded)
}

// Receipt represents a Tableland receipt.
type Receipt struct {
	ChainID       tableland.ChainID
	BlockNumber   int64
	IndexInBlock  int64
	TxnHash       string
	TableIDs      []tables.TableID
	Error         *string
	ErrorEventIdx *int

	// Deprecated: the Receipt must hold information of all tables that were modified by the transaction.
	// This field was replaced by TableIDs.
	TableID *tables.TableID
}

// Table represents a system-wide table stored in Tableland.
type Table struct {
	ID         tables.TableID    `json:"id"` // table id
	ChainID    tableland.ChainID `json:"chain_id"`
	Controller string            `json:"controller"` // controller address
	Prefix     string            `json:"prefix"`
	Structure  string            `json:"structure"`
	CreatedAt  time.Time         `json:"created_at"`
}

// Name returns table's full name.
func (t Table) Name() string {
	return fmt.Sprintf("%s_%d_%s", t.Prefix, t.ChainID, t.ID)
}

// TableSchema represents the schema of a table.
type TableSchema struct {
	Columns          []ColumnSchema
	TableConstraints []string
}

// ColumnSchema represents the schema of a column.
type ColumnSchema struct {
	Name        string
	Type        string
	Constraints []string
}

// TableMetadata represents table metadata (OpenSea standard).
type TableMetadata struct {
	Name         string                   `json:"name,omitempty"`
	ExternalURL  string                   `json:"external_url"`
	Image        string                   `json:"image"`
	Message      string                   `json:"message,omitempty"`
	AnimationURL string                   `json:"animation_url,omitempty"`
	Attributes   []TableMetadataAttribute `json:"attributes,omitempty"`
	Schema       TableSchema              `json:"schema"`
}

// TableMetadataAttribute represents the table metadata attribute.
type TableMetadataAttribute struct {
	DisplayType string      `json:"display_type"`
	TraitType   string      `json:"trait_type"`
	Value       interface{} `json:"value"`
}

// Column defines a column in table data.
type Column struct {
	Name string `json:"name"`
}

// TableData defines a tabular representation of query results.
type TableData struct {
	Columns []Column         `json:"columns"`
	Rows    [][]*ColumnValue `json:"rows"`
}

// ColumnValue wraps data from the db that may be raw json or any other value.
type ColumnValue struct {
	jsonValue  json.RawMessage
	otherValue interface{}
}

// Value returns the underlying value.
func (cv *ColumnValue) Value() interface{} {
	if cv.jsonValue != nil {
		return cv.jsonValue
	}
	return cv.otherValue
}

// Scan implements Scan.
func (cv *ColumnValue) Scan(src interface{}) error {
	cv.jsonValue = nil
	cv.otherValue = nil
	switch src := src.(type) {
	case string:
		trimmed := strings.TrimLeft(src, " ")
		if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(src)) {
			cv.jsonValue = []byte(src)
		} else {
			cv.otherValue = src
		}
	case []byte:
		tmp := make([]byte, len(src))
		copy(tmp, src)
		cv.otherValue = tmp
	default:
		cv.otherValue = src
	}
	return nil
}

// MarshalJSON implements MarshalJSON.
func (cv *ColumnValue) MarshalJSON() ([]byte, error) {
	if cv.jsonValue != nil {
		return cv.jsonValue, nil
	}
	return json.Marshal(cv.otherValue)
}

// JSONColValue creates a UserValue with the provided json.
func JSONColValue(v json.RawMessage) *ColumnValue {
	return &ColumnValue{jsonValue: v}
}

// OtherColValue creates a UserValue with the provided other value.
func OtherColValue(v interface{}) *ColumnValue {
	return &ColumnValue{otherValue: v}
}
