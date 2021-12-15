package tableland

import (
	"context"
)

const ServiceName = "tableland"

type Request struct {
	Table      string
	Controller string
	Statement  string
}

type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type Tableland interface {
	CreateTable(context.Context, Request) (Response, error)
	UpdateTable(context.Context, Request) (Response, error)
	RunSQL(context.Context, Request) (Response, error)
}
