package sqlstore

import (
	"context"
)

// UserStore defines the methods for interacting with user data
type UserStore interface {
	Write(context.Context, string) error
	Read(context.Context, string) (interface{}, error)
}
