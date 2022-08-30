package parsing

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
)

// Stmt represents any valid read or mutating query.
type Stmt interface {
	GetQuery() (string, error)
}

// MutatingStmt represents mutating statement, that is either
// a SugaredWriteStmt or a SugaredGrantStmt.
type MutatingStmt interface {
	Stmt

	// GetPrefix returns the prefix of the table, if any.  e.g: "insert into foo_4_100" -> "foo".
	// Since the prefix is optional, it can return "".
	GetPrefix() string
	// GetTableID returns the table id. "insert into foo_100" -> 100.
	GetTableID() tableland.TableID

	// Operation returns the type of the operation.
	Operation() tableland.Operation

	// GetDBTableName returns the database table name.
	GetDBTableName() string
}

// ReadStmt is an already parsed read statement that satisfies all
// the parser validations. It provides a safe type to use in the business logic
// with correct assumptions about parsing validity and being a read statement
// (select).
type ReadStmt interface {
	Stmt
}

// WriteStmt is an already parsed write statement that satisfies all
// the parser validations. It provides a safe type to use in the business logic
// with correct assumptions about parsing validity and being a write statement
// (update, insert, delete).
type WriteStmt interface {
	MutatingStmt

	// AddWhereClause adds where clauses to update statement.
	AddWhereClause(string) error

	// AddReturningClause add the RETURNING ctid clause to an insert or update statement.
	AddReturningClause() error

	// CheckColumns checks if a column that is not allowed is being touched on update.
	CheckColumns([]string) error
}

// GrantStmt is an already parsed grant statement that satisfies all
// the parser validations. It provides a safe type to use in the business logic
// with correct assumptions about parsing validity and being a write statement
// (grant, revoke).
type GrantStmt interface {
	MutatingStmt

	GetRoles() []common.Address
	GetPrivileges() tableland.Privileges
}

// CreateStmt is a structured create statement. It provides methods to
// help registering and executing the statement correctly.
// Recall that the user sends a create table with the style:
// "create table Person (...)". The real create table query to be executed
// is "create table tXXX (...)".
type CreateStmt interface {
	// GetRawQueryForTableID transforms a parsed create statement
	// from the user, and replaces the referenced table name with
	// the correct name from an id.
	// e.g: "create table Person_69 (...)"(100) -> "create table Person_69_100 (...)".
	GetRawQueryForTableID(tableland.TableID) (string, error)
	// GetStructureHash returns a structure fingerprint of the table, considering
	// the ordered set of columns and types as defined in the spec.
	GetStructureHash() string
	// GetPrefix returns the prefix of the create table.
	// e.g: "create Person_69 (...)" -> "Person".
	GetPrefix() string
}

// SQLValidator parses and validate a SQL query for different supported scenarios.
type SQLValidator interface {
	// ValidateCreateTable validates a CREATE TABLE statement.
	ValidateCreateTable(query string, chainID tableland.ChainID) (CreateStmt, error)
	// ValidateReadQuery validates a read-query, and returns a structured representation of it.
	ValidateReadQuery(query string) (ReadStmt, error)
	// ValidateMutatingQuery validates a mutating-query, and a list of mutating statements
	// contained in it.
	ValidateMutatingQuery(query string, chainID tableland.ChainID) ([]MutatingStmt, error)
}

var (
	// ErrCantAddWhereOnINSERT indicates that the AddWhereClause was called on an insert.
	ErrCantAddWhereOnINSERT = errors.New("can't add where clauses to an insert")

	// ErrCantAddReturningOnDELETE indicates that the AddReturningClause was called on a delete.
	ErrCantAddReturningOnDELETE = errors.New("can't add returning clause to an delete")

	// ErrCanOnlyCheckColumnsOnUPDATE indicates that the CheckColums was called on an insert or delete.
	ErrCanOnlyCheckColumnsOnUPDATE = errors.New("can only check columns on update")
)

// ErrEmptyStatement is an error returned when the statement is empty.
type ErrEmptyStatement struct{}

func (e *ErrEmptyStatement) Error() string {
	return "the statement is empty"
}

// ErrMultiTableReference is an error returned when a multistatement
// references different tables.
type ErrMultiTableReference struct {
	Ref1 string
	Ref2 string
}

func (e *ErrMultiTableReference) Error() string {
	return fmt.Sprintf("queries are referencing two distinct tables: %s %s", e.Ref1, e.Ref2)
}

// ErrSystemTableReferencing is an error returned when queries reference
// system tables which aren't allowed.
type ErrSystemTableReferencing struct {
	ParsingError string
}

func (e *ErrSystemTableReferencing) Error() string {
	strErr := "the query is referencing a system table"
	if e.ParsingError != "" {
		strErr = fmt.Sprintf("%s: %s", strErr, e.ParsingError)
	}
	return strErr
}

// ErrStatementIsNotSupported is an error returned when the stament isn't
// a SELECT, UPDATE, INSERT, DELETE, GRANT or REVOKE.
type ErrStatementIsNotSupported struct{}

func (e *ErrStatementIsNotSupported) Error() string {
	return "the statement isn't supported"
}

// ErrRoleIsNotAnEthAddress is an error returned when the role
// is not an eth address.
type ErrRoleIsNotAnEthAddress struct{}

func (e *ErrRoleIsNotAnEthAddress) Error() string {
	return "role is not an eth address"
}

// ErrNoTopLevelCreate is an error returned when a query isn't a CREATE.
type ErrNoTopLevelCreate struct{}

func (e *ErrNoTopLevelCreate) Error() string {
	return "the query isn't a CREATE"
}

// ErrInvalidTableName is an error returned when a query references a table
// without the right format.
type ErrInvalidTableName struct{}

func (e *ErrInvalidTableName) Error() string {
	return "the query references a table name with the wrong format"
}

// ErrPrefixTableName is an error returned when a query references a table with
// a prefix that is not allowed.
type ErrPrefixTableName struct {
	Prefix string
}

func (e *ErrPrefixTableName) Error() string {
	return fmt.Sprintf("prefix '%s' is not allowed as part of table's name", e.Prefix)
}

// ErrReadQueryTooLong is an error returned when a read query is too long.
type ErrReadQueryTooLong struct {
	Length     int
	MaxAllowed int
}

func (e *ErrReadQueryTooLong) Error() string {
	return fmt.Sprintf("read query size is too long (has %d, max %d)",
		e.Length, e.MaxAllowed)
}

// ErrWriteQueryTooLong is an error returned when a write query is too long.
type ErrWriteQueryTooLong struct {
	Length     int
	MaxAllowed int
}

func (e *ErrWriteQueryTooLong) Error() string {
	return fmt.Sprintf("write query size is too long (has %d, max %d)",
		e.Length, e.MaxAllowed)
}

// Config contains configuration parameters for tableland.
type Config struct {
	MaxReadQuerySize  int
	MaxWriteQuerySize int
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		MaxReadQuerySize:  35000,
		MaxWriteQuerySize: 35000,
	}
}

// Option modifies a configuration attribute.
type Option func(*Config) error

// WithMaxReadQuerySize limits the size of a read query.
func WithMaxReadQuerySize(size int) Option {
	return func(c *Config) error {
		if size <= 0 {
			return fmt.Errorf("size should greater than zero")
		}
		c.MaxReadQuerySize = size
		return nil
	}
}

// WithMaxWriteQuerySize limits the size of a write query.
func WithMaxWriteQuerySize(size int) Option {
	return func(c *Config) error {
		if size <= 0 {
			return fmt.Errorf("size should greater than zero")
		}
		c.MaxWriteQuerySize = size
		return nil
	}
}
