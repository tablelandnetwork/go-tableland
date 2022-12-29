package user

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/XSAM/otelsql"
	_ "github.com/mattn/go-sqlite3" // sqlite3 driver
	logger "github.com/rs/zerolog/log"
	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/metrics"
	"github.com/textileio/go-tableland/pkg/parsing"
	"go.opentelemetry.io/otel/attribute"
)

var log = logger.With().Str("component", "userstore").Logger()

// UserStore provides access to the db store.
type UserStore struct {
	db       *sql.DB
	resolver sqlparser.ReadStatementResolver
}

// New creates a new UserStore.
func New(dbURI string, resolver sqlparser.ReadStatementResolver) (*UserStore, error) {
	attrs := append([]attribute.KeyValue{attribute.String("name", "userstore")}, metrics.BaseAttrs...)
	db, err := otelsql.Open("sqlite3", dbURI, otelsql.WithAttributes(attrs...))
	if err != nil {
		return nil, fmt.Errorf("connecting to db: %s", err)
	}
	if err := otelsql.RegisterDBStatsMetrics(db, otelsql.WithAttributes(attrs...)); err != nil {
		return nil, fmt.Errorf("registering dbstats: %s", err)
	}
	return &UserStore{
		db:       db,
		resolver: resolver,
	}, nil
}

// Read executes a read statement on the db.
func (db *UserStore) Read(ctx context.Context, rq parsing.ReadStmt) (*tableland.TableData, error) {
	query, err := rq.GetQuery(db.resolver)
	if err != nil {
		return nil, fmt.Errorf("get query: %s", err)
	}
	ret, err := execReadQuery(ctx, db.db, query)
	if err != nil {
		return nil, fmt.Errorf("parsing result to json: %s", err)
	}
	return ret, nil
}

// Close closes the store.
func (db *UserStore) Close() error {
	if err := db.db.Close(); err != nil {
		return fmt.Errorf("closing db: %s", err)
	}
	return nil
}

func execReadQuery(ctx context.Context, tx *sql.DB, q string) (*tableland.TableData, error) {
	rows, err := tx.QueryContext(ctx, q)
	if err != nil {
		return nil, fmt.Errorf("executing query: %s", err)
	}
	defer func() {
		if err = rows.Close(); err != nil {
			log.Warn().Err(err).Msg("closing rows")
		}
	}()
	return rowsToTableData(rows)
}
