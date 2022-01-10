package parsing

import "fmt"

// Parser parses and validate a SQL query for different supported scenarios.
type Parser interface {
	ValidateCreateTable(query string) error
	ValidateRunSQL(query string) error
	ValidateReadQuery(query string) error
}

// ErrInvalidSyntax is an error produced when parsing an SQL query.
// The InternalError attribute has the underlying parser error when parsing the query.
type ErrInvalidSyntax struct {
	InternalError error
}

func (eis *ErrInvalidSyntax) Error() string {
	return fmt.Sprintf("unable to parse the query: %s", eis.InternalError)
}

// ErrNoSingleStatement is an error produced when parsing an SQL query detects
// zero or more than one statement.
type ErrNoSingleStatement struct {
}

func (eis *ErrNoSingleStatement) Error() string {
	return "the query contains zero or more than one statement"
}

// ErrNoTopLevelSelect is an error produced when parsing an SQL query detects
// that the query doesn't contain a top-level SELECT statement.
type ErrNoTopLevelSelect struct {
}

func (eis *ErrNoTopLevelSelect) Error() string {
	return "the query isn't a top-level SELECT statement"
}
