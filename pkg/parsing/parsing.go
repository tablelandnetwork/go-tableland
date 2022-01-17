package parsing

import (
	"fmt"

	"github.com/jackc/pgtype"
)

// Parser parses and validate a SQL query for different supported scenarios.
type Parser interface {
	ValidateCreateTable(query string) error
	ValidateRunSQL(query string) error
	ValidateReadQuery(query string) error
}

type TablelandColumnType struct {
	Oid    uint32
	GoType interface{}
	Names  []string
}

var (
	// TODO: this list is tentative and thus incomplete; the accepted types are still not well defined at the spec level.
	AcceptedTypes = map[uint32]TablelandColumnType{
		pgtype.Int2OID: {Oid: pgtype.Int2OID, GoType: &dummyInt, Names: []string{"int2"}},
		pgtype.Int4OID: {Oid: pgtype.Int4OID, GoType: &dummyInt, Names: []string{"int4"}},
		pgtype.Int8OID: {Oid: pgtype.Int8OID, GoType: &dummyInt, Names: []string{"int8"}},

		pgtype.TextOID:    {Oid: pgtype.TextOID, GoType: &dummyStr, Names: []string{"text"}},
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
	}

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

// ErrNoTopLevelSelect is an error returned when the top-level statement isn't
// a SELECT.
type ErrNoTopLevelSelect struct{}

func (e *ErrNoTopLevelSelect) Error() string {
	return "the query isn't a SELECT"
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
