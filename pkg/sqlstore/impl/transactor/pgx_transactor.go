package transactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
)

// Transactor is how you have access to the database pool or the current database transaction.
type Transactor struct {
	pool    *pgxpool.Pool
	tx      *pgx.Tx // current ongoing transaction
	enabled bool
}

// New creates a new Transactor with transaction enabled.
func New(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool, enabled: true}
}

// NewWithoutTransaction creates a new Transactor with transaction disabled.
func NewWithoutTransaction(pool *pgxpool.Pool) *Transactor {
	return &Transactor{pool: pool, enabled: false}
}

// Begin initiates a new transaction.
func (s *Transactor) Begin(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	if s.tx != nil {
		return errors.New("there is an ongoing transaction")
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %s", err)
	}
	s.tx = &tx
	return nil
}

// Commit commits the current ongoing transaction.
func (s *Transactor) Commit(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	if s.tx == nil {
		return errors.New("there is no ongoing transaction")
	}

	err := (*s.tx).Commit(ctx)
	if err != nil {
		return fmt.Errorf("failed to commit transaction: %s", err)
	}
	s.tx = nil
	return nil
}

// Rollback rollbacks the current ongoing transaction.
func (s *Transactor) Rollback(ctx context.Context) error {
	if !s.enabled {
		return nil
	}

	if s.tx == nil {
		return errors.New("there is no ongoing transaction")
	}

	err := (*s.tx).Rollback(ctx)
	if err != nil {
		return fmt.Errorf("failed to rollback transaction: %s", err)
	}
	s.tx = nil
	return nil
}

// Close closes all connections in the pool.
func (s *Transactor) Close() {
	s.pool.Close()
}

// ConnString gets the connections string.
func (s *Transactor) ConnString() string {
	return s.pool.Config().ConnString()
}

// DBTX gets the db connection. It can be inside a transaction or not.
func (s *Transactor) DBTX() DBTX {
	if !s.enabled {
		return s.pool
	}

	if s.tx == nil {
		return s.pool
	}

	return (*s.tx)
}

// DBTX represents the API for interacting with the database.
type DBTX interface {
	Exec(context.Context, string, ...interface{}) (pgconn.CommandTag, error)
	Query(context.Context, string, ...interface{}) (pgx.Rows, error)
	QueryRow(context.Context, string, ...interface{}) pgx.Row
}
