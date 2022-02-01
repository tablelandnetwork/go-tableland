package sqlstore

import (
	"context"
)

// UserStore defines the methods for interacting with user data.
type UserStore interface {
	Read(context.Context, string) (interface{}, error)
}
