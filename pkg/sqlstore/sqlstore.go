package sqlstore

type SQLStore interface {
	Query(string) error
}
