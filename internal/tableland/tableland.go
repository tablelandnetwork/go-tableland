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

// RunSQLRequest is a user RunSQL request.
type RunSQLRequest struct {
	Controller string `json:"controller"`
	Statement  string `json:"statement"`
}

// RunSQLResponse is a RunSQL response.
type RunSQLResponse struct {
	Result interface{} `json:"data"`
}

// AuthorizeRequest is a user Authorize request.
type AuthorizeRequest struct {
	Controller string `json:"controller"`
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	CreateTable(context.Context, CreateTableRequest) (CreateTableResponse, error)
	RunSQL(context.Context, RunSQLRequest) (RunSQLResponse, error)
	Authorize(context.Context, AuthorizeRequest) error
}
