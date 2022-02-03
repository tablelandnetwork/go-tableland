package parsing

import (
	"fmt"
	"math/big"

	"github.com/jackc/pgtype"
	"github.com/textileio/go-tableland/internal/tableland"
)

type Stmt interface {
	GetRawQuery() string
}

// WriteStmt is an already parsed write statement that satisfies all
// the parser validations.
type WriteStmt interface {
	Stmt
}

// ReadStmt is an already parsed read statement that satisfies all
// the parser validations.
type ReadStmt interface {
	Stmt
}

type CreateStmt interface {
	GetRawQueryForTableID(*big.Int) (string, error)
	GetStructureHash() string
	GetNamePrefix() string
}

// SQLValidator parses and validate a SQL query for different supported scenarios.
type SQLValidator interface {
	// ValidateCreateTable validates the provided query and returns an error
	// if the CREATE statement isn't allowed. Returns nil otherwise.
	ValidateCreateTable(query string) (CreateStmt, error)
	// ValidateRunSQL validates the query and returns an error if isn't allowed.
	// It returns the table ID extracted from the query, and a read *or* write
	// statement depending on the query type.
	ValidateRunSQL(query string) (tableland.TableID, ReadStmt, []WriteStmt, error)
}

// TablelandColumnType represents an accepted column type for user-tables.
type TablelandColumnType struct {
	// Oid is the corresponding postgres datatype OID.
	Oid uint32
	// GoType contains a value of the correct type to be used for
	// json unmarshaling.
	GoType interface{}
	// Names contains a list of postgres datatype names to be used by the parser
	// to recognize the column type.
	Names []string
}

var (
	// AcceptedTypes contains all the accepted column types in user-defined tables.
	// It's used by the parser and the JSON marshaler to validate queries, and transform to appropriate
	// Go types respectively.
	AcceptedTypes = map[uint32]TablelandColumnType{
		pgtype.Int2OID: {Oid: pgtype.Int2OID, GoType: &dummyInt, Names: []string{"int2"}},
		pgtype.Int4OID: {Oid: pgtype.Int4OID, GoType: &dummyInt, Names: []string{"int4", "serial"}},
		pgtype.Int8OID: {Oid: pgtype.Int8OID, GoType: &dummyInt, Names: []string{"int8", "bigserial"}},

		pgtype.TextOID:    {Oid: pgtype.TextOID, GoType: &dummyStr, Names: []string{"text", "uri"}},
		pgtype.VarcharOID: {Oid: pgtype.VarcharOID, GoType: &dummyStr, Names: []string{"varchar"}},
		pgtype.BPCharOID:  {Oid: pgtype.BPCharOID, GoType: &dummyStr, Names: []string{"bpchar"}},

		pgtype.DateOID: {Oid: pgtype.DateOID, GoType: pgtype.Date{}, Names: []string{"date"}},

		pgtype.BoolOID: {Oid: pgtype.BoolOID, GoType: &dummyBool, Names: []string{"bool"}},

		pgtype.Float4OID: {Oid: pgtype.Float4OID, GoType: &dummyFloat64, Names: []string{"float4"}},
		pgtype.Float8OID: {Oid: pgtype.Float8OID, GoType: &dummyFloat64, Names: []string{"float8"}},

		pgtype.NumericOID: {Oid: pgtype.NumericOID, GoType: pgtype.Numeric{}, Names: []string{"numeric"}},

		pgtype.TimestampOID: {Oid: pgtype.TimestampOID, GoType: pgtype.Timestamp{}, Names: []string{"timestamp"}},

		pgtype.TimestamptzOID: {Oid: pgtype.TimestamptzOID, GoType: pgtype.Timestamptz{}, Names: []string{"timestamptz"}},

		pgtype.UUIDOID: {Oid: pgtype.UUIDOID, GoType: pgtype.UUID{}, Names: []string{"uuid"}},

		pgtype.JSONOID: {Oid: pgtype.JSONOID, GoType: map[string]interface{}{}, Names: []string{"json"}},
	}
	// TODO: the above list is tentative and incomplete; the accepted types are still not well defined at the spec level.

	dummyInt     int
	dummyStr     string
	dummyBool    bool
	dummyFloat64 float64
)

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

// ErrEmptyStatement is an error returned when the statement is empty.
type ErrEmptyStatement struct{}

func (e *ErrEmptyStatement) Error() string {
	return "the statement is empty"
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
	strErr := "the query is referencing a system table"
	if e.ParsingError != "" {
		strErr = fmt.Sprintf("%s: %s", strErr, e.ParsingError)
	}
	return strErr
}

// ErrNoTopLevelUpdateInsertDelete is an error returned the query isn't
// an UPDATE, INSERT or DELETE.
type ErrNoTopLevelUpdateInsertDelete struct{}

func (e *ErrNoTopLevelUpdateInsertDelete) Error() string {
	return "the query isn't a an UPDATE, INSERT, or DELETE"
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

// ErrNoTopLevelCreate is an error returned when a query isn't a CREATE.
type ErrNoTopLevelCreate struct{}

func (e *ErrNoTopLevelCreate) Error() string {
	return "the query isn't a CREATE"
}

// ErrInvalidColumnType is an error returned when a table is created
// with a disallowed column type.
type ErrInvalidColumnType struct {
	ColumnType string
}

func (e *ErrInvalidColumnType) Error() string {
	str := "the created table has an invalid column type"
	if e.ColumnType != "" {
		str = fmt.Sprintf("%s: %s", str, e.ColumnType)
	}
	return str
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

// ErrInvalidTableName is an error returned when a query references a table
// without the right format.
type ErrInvalidTableName struct{}

func (e *ErrInvalidTableName) Error() string {
	return "the query references a table name with the wrong format"
}
