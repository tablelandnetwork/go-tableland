package sqlstore

import (
	"context"

	"github.com/textileio/go-tableland/pkg/parsing"
)

// UserColumn defines a column in a row result.
type UserColumn struct {
	Name string `json:"name"`
}

// UserRows defines a row result.
type UserRows struct {
	Columns []UserColumn    `json:"columns"`
	Rows    [][]interface{} `json:"rows"`
}

// UserStore defines the methods for interacting with user data.
type UserStore interface {
	Read(context.Context, parsing.SugaredReadStmt) (interface{}, error)
}
