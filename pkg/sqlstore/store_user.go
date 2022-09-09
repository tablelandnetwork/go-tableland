package sqlstore

import (
	"context"

	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
)

// UserStore defines the methods for interacting with user data.
type UserStore interface {
	Read(context.Context, parsing.ReadStmt) (*tableland.TableData, error)
	Close() error
}
