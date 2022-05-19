package parsing

import (
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgtype"
	"github.com/textileio/go-tableland/internal/tableland"
)

// SugaredStmt is a structured statement. It's "sugared" since the table
// references are {name}_t{ID} ({name)_ is optional).
// It provides methods that helps with validations and execution in the real Tableland
// database, since sugared queries should be desugared for correct execution.
type SugaredStmt interface {
	// TODO(jsign): fix comments.
	// GetDesugared query desugars the query, which means:
	// "insert into foo_100" -> "insert _100"
	GetDesugaredQuery() (string, error)
}

// SugaredWriteStmt is an already parsed write statement that satisfies all
// the parser validations. It provides a safe type to use in the business logic
// with correct assumptions about parsing validity and being a write statement
// (update, insert, delete).
type SugaredWriteStmt interface {
	SugaredMutatingStmt

	// AddWhereClause adds where clauses to update statement.
	AddWhereClause(string) error

	// AddReturningClause add the RETURNING ctid clause to an insert or update statement.
	AddReturningClause() error

	// CheckColumns checks if a column that is not allowed is being touched on update.
	CheckColumns([]string) error
}

// SugaredGrantStmt is an already parsed grant statement that satisfies all
// the parser validations. It provides a safe type to use in the business logic
// with correct assumptions about parsing validity and being a write statement
// (grant, revoke).
type SugaredGrantStmt interface {
	SugaredMutatingStmt
	GetRoles() []common.Address
	GetPrivileges() tableland.Privileges
}

// SugaredMutatingStmt represents mutating statement, that is either
// a SugaredWriteStmt or a SugaredGrantStmt.
type SugaredMutatingStmt interface {
	SugaredStmt

	// GetNamePrefix returns the name prefix of the sugared table name
	// if exists. e.g: "insert into foo_100" -> "foo". Since the name
	// prefix is optional, it can return "" if none exist in the query.
	GetNamePrefix() string
	// GetTableID returns the table id. "insert into foo_100" -> 100.
	GetTableID() tableland.TableID

	// Operation returns the type of the operation
	Operation() tableland.Operation

	// GetDBTableName returns the database table name
	GetDBTableName() string
}

// SugaredReadStmt is an already parsed read statement that satisfies all
// the parser validations. It provides a safe type to use in the business logic
// with correct assumptions about parsing validity and being a read statement
// (select).
type SugaredReadStmt interface {
	SugaredStmt
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
	// e.g: "create table Person (...)"(100) -> "create table t100 (...)".
	GetRawQueryForTableID(tableland.TableID) (string, error)
	// GetStructureHash returns a structure fingerprint of the table, considering
	// the ordered set of columns and types as defined in the spec.
	GetStructureHash() string
	// GetNamePrefix returns the sugared name from the user query.
	// e.g: "create Person (...)" -> "Person". This helps to feed the
	// system tables "name" corresponding column.
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
	ValidateRunSQL(query string) (SugaredReadStmt, []SugaredMutatingStmt, error)
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

		pgtype.JSONOID: {Oid: pgtype.JSONOID, GoType: pgtype.JSON{}, Names: []string{"json"}},
	}
	// TODO: the above list is tentative and incomplete; the accepted types are still not well defined at the spec level.

	dummyInt     int
	dummyStr     string
	dummyBool    bool
	dummyFloat64 float64

	// ErrCantAddWhereOnINSERT indicates that the AddWhereClause was called on an insert.
	ErrCantAddWhereOnINSERT = errors.New("can't add where clauses to an insert")

	// ErrCantAddReturningOnDELETE indicates that the AddReturningClause was called on a delete.
	ErrCantAddReturningOnDELETE = errors.New("can't add returning clause to an delete")

	// ErrCanOnlyCheckColumnsOnUPDATE indicates that the CheckColums was called on an insert or delete.
	ErrCanOnlyCheckColumnsOnUPDATE = errors.New("can only check columns on update")
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

// ErrStatementIsNotSupported is an error returned when the stament isn't
// a SELECT, UPDATE, INSERT, DELETE, GRANT or REVOKE.
type ErrStatementIsNotSupported struct{}

func (e *ErrStatementIsNotSupported) Error() string {
	return "the statement isn't supported"
}

// ErrNoTopLevelGrant is an error returned when the query isn't
// a GRANT or REVOKE.
type ErrNoTopLevelGrant struct{}

func (e *ErrNoTopLevelGrant) Error() string {
	return "the query isn't a an GRANT or REVOKE"
}

// ErrAllPrivilegesNotAllowed is an error returned when the grant
// is ALL PRIVILEGES.
type ErrAllPrivilegesNotAllowed struct{}

func (e *ErrAllPrivilegesNotAllowed) Error() string {
	return "ALL PRIVILEGES is not allowed"
}

// ErrNoInsertUpdateDeletePrivilege is an error returned when the privilege isn't
// an UPDATE, INSERT or DELETE.
type ErrNoInsertUpdateDeletePrivilege struct{}

func (e *ErrNoInsertUpdateDeletePrivilege) Error() string {
	return "the privilege can only be INSERT, UPDATE or DELETE"
}

// ErrNoSingleTableReference is an error returned when the grant isn't
// referencing only one table.
type ErrNoSingleTableReference struct{}

func (e *ErrNoSingleTableReference) Error() string {
	return "grant can only reference one table"
}

// ErrObjectTypeIsNotTable is an error returned when the grant isn't
// referencing a table.
type ErrObjectTypeIsNotTable struct{}

func (e *ErrObjectTypeIsNotTable) Error() string {
	return "grant can only reference object of type OBJECT_TABLE"
}

// ErrRangeVarIsNil is an error returned when the grant RangeVar is nil.
type ErrRangeVarIsNil struct{}

func (e *ErrRangeVarIsNil) Error() string {
	return "grant rangevar is nil"
}

// ErrRoleIsNotCString is an error returned when the rolespec
// of the role is not cstring.
type ErrRoleIsNotCString struct{}

func (e *ErrRoleIsNotCString) Error() string {
	return "rolespec if not of type cstring"
}

// ErrRoleIsNotAnEthAddress is an error returned when the role
// is not an eth address.
type ErrRoleIsNotAnEthAddress struct{}

func (e *ErrRoleIsNotAnEthAddress) Error() string {
	return "role is not an eth address"
}

// ErrTargetTypeIsNotObject is an error returned when the target type
// is not object.
type ErrTargetTypeIsNotObject struct{}

func (e *ErrTargetTypeIsNotObject) Error() string {
	return "target type is not ACL_TARGET_OBJECT"
}

// ErrReturningClause is an error returned when queries use a RETURNING clause.
type ErrReturningClause struct{}

func (e *ErrReturningClause) Error() string {
	return "the query uses a RETURNING clause"
}

// ErrRelationAlias is an error returned when queries use alias on relation.
type ErrRelationAlias struct{}

func (e *ErrRelationAlias) Error() string {
	return "the query uses an alias for relation"
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

// ErrTooManyColumns is an error returned when a create statement has
// more columns that allowed.
type ErrTooManyColumns struct {
	ColumnCount int
	MaxAllowed  int
}

func (e *ErrTooManyColumns) Error() string {
	return fmt.Sprintf("table has too many columns (has %d, max %d)",
		e.ColumnCount, e.MaxAllowed)
}

// ErrTextTooLong is an error returned when a write query contains a
// text constant that is too long.
type ErrTextTooLong struct {
	Length     int
	MaxAllowed int
}

func (e *ErrTextTooLong) Error() string {
	return fmt.Sprintf("text field length is too long (has %d, max %d)",
		e.Length, e.MaxAllowed)
}
