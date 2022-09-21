package dbhash

import (
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
)

// DatabaseStateHash calculates the hash of some state of the database according to the options passed.
func DatabaseStateHash(ctx context.Context, tx *sql.Tx, opts ...Option) (string, error) {
	config := DefaultConfig()
	for _, o := range opts {
		if err := o(config); err != nil {
			return "", fmt.Errorf("applying provided option: %s", err)
		}
	}

	h := sha1.New()
	if err := databaseStateWriter(ctx, tx, h, config); err != nil {
		return "", fmt.Errorf("database hash writer failed: %s", err)
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func databaseStateWriter(ctx context.Context, tx *sql.Tx, writer io.Writer, c *Config) error {
	// get all tables from db and associated schema
	rows, err := tx.QueryContext(ctx, c.FetchSchemasQuery)
	if err != nil {
		return fmt.Errorf("querying schemas: %s", err)
	}
	defer func() {
		_ = rows.Close()
	}()

	for rows.Next() {
		var name, stmt string
		if err = rows.Scan(
			&name,
			&stmt,
		); err != nil {
			return fmt.Errorf("schema scan row: %s", err)
		}

		_, _ = writer.Write([]byte(name))
		_, _ = writer.Write([]byte(stmt))

		if err := tableStateWriter(ctx, tx, name, writer, c); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			return fmt.Errorf("table state writer: %s", err)
		}
	}

	return nil
}

func tableStateWriter(ctx context.Context, tx *sql.Tx, name string, writer io.Writer, c *Config) error {
	tableRows, err := tx.QueryContext(ctx, c.PerTableQueryFn(name))
	if err != nil {
		return fmt.Errorf("querying table: %v", err)
	}
	defer func() {
		_ = tableRows.Close()
	}()

	cols, err := tableRows.Columns()
	if err != nil {
		return fmt.Errorf("columns: %s", err)
	}

	rawBuffer := make([]sql.RawBytes, len(cols))
	scanCallArgs := make([]interface{}, len(rawBuffer))
	for i := range rawBuffer {
		scanCallArgs[i] = &rawBuffer[i]
	}

	for tableRows.Next() {
		if err := tableRows.Scan(scanCallArgs...); err != nil {
			return fmt.Errorf("table row scan: %s", err)
		}

		for _, col := range rawBuffer {
			_, _ = writer.Write(col)
		}
	}

	return nil
}

// Config contains configuration parameters for tableland.
type Config struct {
	FetchSchemasQuery string
	PerTableQueryFn   func(string) string
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		FetchSchemasQuery: "SELECT tbl_name, sql FROM sqlite_schema WHERE type='table' ORDER BY tbl_name",
		PerTableQueryFn: func(tableName string) string {
			return fmt.Sprintf("SELECT * FROM %s", tableName)
		},
	}
}

// Option modifies a configuration attribute.
type Option func(*Config) error

// WithFetchSchemasQuery limits tables that will be used for hash calculation.
func WithFetchSchemasQuery(clause string) Option {
	return func(c *Config) error {
		c.FetchSchemasQuery = clause
		return nil
	}
}

// WithPerTableQueryFn defines a function that returns a query for a given table.
func WithPerTableQueryFn(fn func(tableName string) string) Option {
	return func(c *Config) error {
		c.PerTableQueryFn = fn
		return nil
	}
}
