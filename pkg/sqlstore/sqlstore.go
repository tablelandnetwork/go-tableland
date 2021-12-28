package sqlstore

import (
	"context"
)

// SQLStore defines the methods for interacting with Tableland storage.
type SQLStore interface {
	UserStore
	SystemStore
	Begin(context.Context) error
	Commit(context.Context) error
	Rollback(context.Context) error
	Close()
}
