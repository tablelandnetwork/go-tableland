package tableland

import "context"

const ServiceName = "tableland"

type SQLArgs struct {
	Statement string
}

type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type Tableland interface {
	CreateTable(context.Context, SQLArgs) (Response, error)
	UpdateTable(context.Context, SQLArgs) (Response, error)
	RunSQL(context.Context, SQLArgs) (Response, error)
}
