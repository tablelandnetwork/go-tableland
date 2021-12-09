package tableland

const ServiceName = "tableland"

type SQLArgs struct {
	Statement string
}

type Response struct {
	Message string      `json:"message"`
	Data    interface{} `json:"data"`
}

type Tableland interface {
	CreateTable(SQLArgs) (Response, error)
	UpdateTable(SQLArgs) (Response, error)
	RunSQL(SQLArgs) (Response, error)
}
