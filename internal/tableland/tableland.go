package tableland

import (
	"context"
)

// CreateTableRequest is a user CreateTable request.
type CreateTableRequest struct {
	ID          string `json:"id"`
	Controller  string `json:"controller"`
	Statement   string `json:"statement"`
	Description string `json:"description"`
}

// CreateTableResponse is a CreateTable response.
type CreateTableResponse struct {
	Tablename string `json:"tablename"`
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	CreateTable(context.Context, CreateTableRequest) (CreateTableResponse, error)
	RunSQL(context.Context, RunSQLRequest) (RunSQLResponse, error)
	Authorize(context.Context, AuthorizeRequest) error
}
