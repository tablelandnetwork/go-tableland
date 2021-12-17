package tableland

import (
	"context"
)

// Request is a user request to interact with Tableland.
type Request struct {
	TableID    string
	Controller string
	Statement  string
}

// Response is a response to a Tableland request.
type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

// Tableland defines the interface of Tableland.
type Tableland interface {
	CreateTable(context.Context, Request) (Response, error)
	UpdateTable(context.Context, Request) (Response, error)
	RunSQL(context.Context, Request) (Response, error)
}
