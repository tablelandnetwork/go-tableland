package sqlstore

import (
	"context"
	"encoding/json"
	"strings"

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

// UserData wraps data from the db that may be raw json or any other value.
type UserData struct {
	jsonValue  json.RawMessage
	otherValue interface{}
}

// Value returns the underlying value.
func (u *UserData) Value() interface{} {
	if u.jsonValue != nil {
		return u.jsonValue
	}
	return u.otherValue
}

// Scan implements Scan.
func (u *UserData) Scan(src interface{}) error {
	u.jsonValue = nil
	u.otherValue = nil
	switch src := src.(type) {
	case string:
		if (strings.HasPrefix(src, "{") || strings.HasPrefix(src, "[")) && json.Valid([]byte(src)) {
			u.jsonValue = []byte(src)
		} else {
			u.otherValue = src
		}
	case []byte:
		tmp := src
		u.otherValue = tmp
	default:
		u.otherValue = src
	}
	return nil
}

// MarshalJSON implements MarshalJSON.
func (u *UserData) MarshalJSON() ([]byte, error) {
	if u.jsonValue != nil {
		return u.jsonValue, nil
	}
	return json.Marshal(u.otherValue)
}

// UserStore defines the methods for interacting with user data.
type UserStore interface {
	Read(context.Context, parsing.ReadStmt) (interface{}, error)
	Close() error
}
