package sqlstore

// SQLStore defines the methods for interacting with Tableland storage
type SQLStore interface {
	UserStore
	SystemStore
	Close()
}
