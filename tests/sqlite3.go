package tests

import (
	"github.com/google/uuid"
)

func Sqlite3URL() string {
	return "file::" + uuid.NewString() + ":?mode=memory&cache=shared&_foreign_keys=on"
}
