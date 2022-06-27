package tests

import (
	"github.com/google/uuid"
)

// Sqlite3URI returns a URI to spinup an in-memory Sqlite database.
func Sqlite3URI() string {
	return "file::" + uuid.NewString() + ":?mode=memory&cache=shared&_foreign_keys=on"
}
