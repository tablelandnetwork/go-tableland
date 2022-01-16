package impl_test

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/parsing"
	postgresparser "github.com/textileio/go-tableland/internal/parsing/impl"
)

func TestReadQuery(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		query           string
		expectedErrType interface{}
	}
	tests := []testCase{
		// Malformed query.
		{name: "malformed query", query: "shelect * from foo", expectedErrType: ptr2ErrInvalidSyntax()},

		// Valid read-queries.
		{name: "valid all", query: "select * from foo", expectedErrType: nil},
		{name: "valid defined rows", query: "select row1, row2 from foo", expectedErrType: nil},
		{name: "valid join with subquery", query: "select * from foo inner join bar on a=b inner join (select * from zoo) z on a=b", expectedErrType: nil},

		// Single-statement check.
		{name: "single statement fail", query: "select * from foo; select * from bar", expectedErrType: ptr2ErrNoSingleStatement()},
		{name: "no statements", query: "", expectedErrType: ptr2ErrNoSingleStatement()},

		// Check top-statement is only SELECT.
		{name: "create", query: "create table foo (bar int)", expectedErrType: ptr2ErrNoTopLevelSelect()},
		{name: "update", query: "update foo set bar=1", expectedErrType: ptr2ErrNoTopLevelSelect()},
		{name: "insert", query: "insert into foo values (1)", expectedErrType: ptr2ErrNoTopLevelSelect()},
		{name: "drop", query: "drop table foo", expectedErrType: ptr2ErrNoTopLevelSelect()},
		{name: "delete", query: "delete from foo", expectedErrType: ptr2ErrNoTopLevelSelect()},

		// Check no FROM SHARE/UPDATE
		{name: "for share", query: "select * from foo for share", expectedErrType: ptr2ErrNoForUpdateOrShare()},
		{name: "for update", query: "select * from foo for update", expectedErrType: ptr2ErrNoForUpdateOrShare()},

		// Check no system-tables references.
		{name: "reference system table", query: "select * from system_tables", expectedErrType: ptr2ErrSystemTableReferencing()},
		{name: "reference system table with inner join", query: "select * from foo inner join system_tables on a=b", expectedErrType: ptr2ErrSystemTableReferencing()},
		{name: "reference system table in nested FROM SELECT", query: "select * from foo inner join (select * from system_tables) j on a=b", expectedErrType: ptr2ErrSystemTableReferencing()},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New("system_")
				err := parser.ValidateReadQuery(tc.query)
				if tc.expectedErrType == nil {
					require.NoError(t, err)
					return
				}
				require.ErrorAs(t, err, tc.expectedErrType)
			}
		}(it))
	}
}

func TestRunSQL(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		query           string
		expectedErrType interface{}
	}
	tests := []testCase{
		// Malformed query.
		{name: "malformed insert", query: "insert into foo valuez (1, 1)", expectedErrType: ptr2ErrInvalidSyntax()},
		{name: "malformed update", query: "update foo sez a=1, b=2", expectedErrType: ptr2ErrInvalidSyntax()},
		{name: "malformed delete", query: "delete fromz foo where a=2", expectedErrType: ptr2ErrInvalidSyntax()},

		// Valid insert and updates.
		{name: "valid insert", query: "insert into foo values ('hello', 1, 2)", expectedErrType: nil},
		{name: "valid simple update", query: "update foo set a=1 where b='hello'", expectedErrType: nil},
		{name: "valid joined update", query: "update foo set c=1 from bar where foo.a=bar.b", expectedErrType: nil},
		{name: "valid delete", query: "delete from foo where a=2", expectedErrType: nil},
		//{name: "valid custom func call", query: "insert into foo values (myfunc(1))", expectedErrType: nil},

		// Single-statement check.
		{name: "single statement fail", query: "update foo set a=1; update foo set b=1;", expectedErrType: ptr2ErrNoSingleStatement()},
		{name: "no statements", query: "", expectedErrType: ptr2ErrNoSingleStatement()},

		// Check top-statement are INSERT, UPDATE and DELETE.
		{name: "create", query: "create table foo (bar int)", expectedErrType: ptr2ErrNoTopLevelUpdateInsertDelete()},
		{name: "select", query: "select * from foo", expectedErrType: ptr2ErrNoTopLevelUpdateInsertDelete()},
		{name: "drop", query: "drop table foo", expectedErrType: ptr2ErrNoTopLevelUpdateInsertDelete()},

		// Disallow RETURNING clauses
		{name: "update returning", query: "update foo set a=a+1 returning a", expectedErrType: ptr2ErrReturningClause()},
		{name: "insert returning", query: "insert into foo values (1, 'bar') returning a", expectedErrType: ptr2ErrReturningClause()},
		{name: "delete returning", query: "delete from foo where a=1 returning b", expectedErrType: ptr2ErrReturningClause()},

		// Check no system-tables references.
		{name: "update system table", query: "update system_tables set a=1", expectedErrType: ptr2ErrSystemTableReferencing()},
		{name: "insert system table", query: "insert into system_tables values ('foo')", expectedErrType: ptr2ErrSystemTableReferencing()},
		{name: "delete system table", query: "delete from system_tables", expectedErrType: ptr2ErrSystemTableReferencing()},
		{name: "update referencing system table with from", query: "update foo set a=1 from system_tables where a=b", expectedErrType: ptr2ErrSystemTableReferencing()},
		{name: "reference system table in nested from", query: "update foo set a=1 from (select * from system_tables) st where st.a=foo.b", expectedErrType: ptr2ErrSystemTableReferencing()},

		// Check non-deterministic functions.
		{name: "insert current_timestamp lower", query: "insert into foo values (current_timestamp, 'lolz')", expectedErrType: ptr2ErrNonDeterministicFunction()},
		{name: "insert current_timestamp case-insensitive", query: "insert into foo values (current_TiMeSTamP, 'lolz')", expectedErrType: ptr2ErrNonDeterministicFunction()},
		{name: "update set current_timestamp", query: "update foo set a=current_timestamp, b=2", expectedErrType: ptr2ErrNonDeterministicFunction()},
		{name: "update where current_timestamp", query: "update foo set a=1 where b=current_timestamp", expectedErrType: ptr2ErrNonDeterministicFunction()},
		{name: "delete where current_timestamp", query: "delete from foo where a=current_timestamp", expectedErrType: ptr2ErrNonDeterministicFunction()},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New("system_")
				err := parser.ValidateRunSQL(tc.query)
				if tc.expectedErrType == nil {
					require.NoError(t, err)
					return
				}
				require.ErrorAs(t, err, tc.expectedErrType)
			}
		}(it))
	}
}

// Helpers to have a pointer to pointer for generic test-case running.
func ptr2ErrInvalidSyntax() **parsing.ErrInvalidSyntax {
	e := &parsing.ErrInvalidSyntax{}
	return &e
}
func ptr2ErrNoSingleStatement() **parsing.ErrNoSingleStatement {
	e := &parsing.ErrNoSingleStatement{}
	return &e
}
func ptr2ErrNoTopLevelSelect() **parsing.ErrNoTopLevelSelect {
	e := &parsing.ErrNoTopLevelSelect{}
	return &e
}
func ptr2ErrNoForUpdateOrShare() **parsing.ErrNoForUpdateOrShare {
	e := &parsing.ErrNoForUpdateOrShare{}
	return &e
}
func ptr2ErrSystemTableReferencing() **parsing.ErrSystemTableReferencing {
	e := &parsing.ErrSystemTableReferencing{}
	return &e
}
func ptr2ErrNoTopLevelUpdateInsertDelete() **parsing.ErrNoTopLevelUpdateInsertDelete {
	e := &parsing.ErrNoTopLevelUpdateInsertDelete{}
	return &e
}
func ptr2ErrReturningClause() **parsing.ErrReturningClause {
	e := &parsing.ErrReturningClause{}
	return &e
}
func ptr2ErrNonDeterministicFunction() **parsing.ErrNonDeterministicFunction {
	e := &parsing.ErrNonDeterministicFunction{}
	return &e
}
