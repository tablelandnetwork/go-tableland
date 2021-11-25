package sqlstore

type SQLStore interface {
	createTable(string) error
	updateTable(string) error
	runSql()
}
