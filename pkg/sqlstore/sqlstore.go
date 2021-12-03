package sqlstore

type SQLStore interface {
	Write(string) error
	Read(string) (interface{}, error)
	Close()
}
