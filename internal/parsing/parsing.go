package parsing

import "fmt"

// Parser parses and validate a SQL query for different supported scenarios.
type Parser interface {
	ValidateCreateTable(query string) error
	ValidateRunSQL(query string) error
	ValidateReadQuery(query string) error
}

// ErrInvalidSyntax is an error returned when parsing the query.
// The InternalError attribute has the underlying parser error when parsing the query.
type ErrInvalidSyntax struct {
	InternalError error
}

func (e *ErrInvalidSyntax) Error() string {
	return fmt.Sprintf("unable to parse the query: %s", e.InternalError)
}

// ErrNoSingleStatement is an error returned when there isn't a single statement.
type ErrNoSingleStatement struct{}

func (e *ErrNoSingleStatement) Error() string {
	return "the query contains zero or more than one statement"
}

// ErrNoTopLevelSelect is an error returned when the top-level statement isn't
// a SELECT.
type ErrNoTopLevelSelect struct{}

func (e *ErrNoTopLevelSelect) Error() string {
	return "the query isn't a top-level SELECT statement"
}

// ErrNoForUpdateOrShare is an error returned when a SELECT statements use
// a FOR UPDATE/SHARE clause.
type ErrNoForUpdateOrShare struct{}

func (e *ErrNoForUpdateOrShare) Error() string {
	return "FOR UPDATE/SHARE isn't allowed"
}

// ErrSystemTableReferencing is an error returned when queries reference
// system tables which aren't allowed.
type ErrSystemTableReferencing struct {
	ParsingError string
}

func (e *ErrSystemTableReferencing) Error() string {
	if e.ParsingError != "" {
		return fmt.Sprintf("system table reference checking errored: %s", e.ParsingError)
	}
	return "the query is referencing a system table"
}

// ErrNoTopLevelUpdateInsertDelete is an error returned when the top-level statement isn't
// an UPDATE, INSERT or DELETE.
type ErrNoTopLevelUpdateInsertDelete struct{}

func (e *ErrNoTopLevelUpdateInsertDelete) Error() string {
	return "the query isn't a top-level UPDATE, INSERT, or DELETE"
}

// ErrReturningClause is an error returned when queries use a RETURNING clause.
type ErrReturningClause struct{}

func (e *ErrReturningClause) Error() string {
	return "the query uses a RETURNING clause"
}

// ErrNonDeterministicFunction is an error returned when queries use non-deterministic
// function.
type ErrNonDeterministicFunction struct{}

func (e *ErrNonDeterministicFunction) Error() string {
	return "the query uses a non-deterministic function"
}

// ErrJoinOrSubquery is an error returned when queries uses JOINs or
// subqueries.
type ErrJoinOrSubquery struct{}

func (e *ErrJoinOrSubquery) Error() string {
	return "the query uses a join or subquery"
}
