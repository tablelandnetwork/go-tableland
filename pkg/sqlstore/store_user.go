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
	Columns []UserColumn   `json:"columns"`
	Rows    [][]*UserValue `json:"rows"`
}

// UserValue wraps data from the db that may be raw json or any other value.
type UserValue struct {
	jsonValue  json.RawMessage
	otherValue interface{}
}

// Value returns the underlying value.
func (u *UserValue) Value() interface{} {
	if u.jsonValue != nil {
		return u.jsonValue
	}
	return u.otherValue
}

// Scan implements Scan.
func (u *UserValue) Scan(src interface{}) error {
	u.jsonValue = nil
	u.otherValue = nil
	switch src := src.(type) {
	case string:
		trimmed := strings.TrimLeft(src, " ")
		if (strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[")) && json.Valid([]byte(src)) {
			u.jsonValue = []byte(src)
		} else {
			u.otherValue = src
		}
	case []byte:
		tmp := make([]byte, len(src))
		copy(tmp, src)
		u.otherValue = tmp
	default:
		u.otherValue = src
	}
	return nil
}

// MarshalJSON implements MarshalJSON.
func (u *UserValue) MarshalJSON() ([]byte, error) {
	if u.jsonValue != nil {
		return u.jsonValue, nil
	}
	return json.Marshal(u.otherValue)
}

// JSONUserValue creates a UserValue with the provided json.
func JSONUserValue(v json.RawMessage) *UserValue {
	return &UserValue{jsonValue: v}
}

// OtherUserValue creates a UserValue with the provided other value.
func OtherUserValue(v interface{}) *UserValue {
	return &UserValue{otherValue: v}
}

// UserStore defines the methods for interacting with user data.
type UserStore interface {
	Read(context.Context, parsing.ReadStmt) (interface{}, error)
	Close() error
}
