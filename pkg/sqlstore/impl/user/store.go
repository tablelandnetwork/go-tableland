package user

import (
	"context"

	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/transactor"
)

// UserStore provides access to the db store.
type UserStore struct {
	t *transactor.Transactor
}

// New creates a new UserStore.
func New(transactor *transactor.Transactor) *UserStore {
	return &UserStore{transactor}
}

// Write executes a write statement on the db.
func (s *UserStore) Write(ctx context.Context, statement string) error {
	_, err := s.t.DBTX().Exec(ctx, statement)
	return err
}

// Read executes a read statement on the db.
func (s *UserStore) Read(ctx context.Context, statement string) (interface{}, error) {
	rows, err := s.t.DBTX().Query(ctx, statement, pgx.QuerySimpleProtocol(true))
	if err != nil {
		return []byte{}, err
	}

	defer rows.Close()
	return rowsToJSON(rows)
}
