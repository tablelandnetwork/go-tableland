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
		{name: "malformed query", query: "shelect * from foo", expectedErrType: &parsing.ErrInvalidSyntax{}},

		// Valid read-queries.
		{name: "valid all", query: "select * from foo", expectedErrType: nil},
		{name: "valid defined rows", query: "select row1, row2 from foo", expectedErrType: nil},

		// Single-statement check.
		{name: "single statement fail", query: "select * from foo; select * from bar", expectedErrType: &parsing.ErrNoSingleStatement{}},
		{name: "no statements", query: "", expectedErrType: &parsing.ErrNoSingleStatement{}},

		// Check top-statement is only SELECT.
		{name: "create", query: "create table foo (bar int)", expectedErrType: &parsing.ErrNoTopLevelSelect{}},
		{name: "update", query: "update foo set bar=1", expectedErrType: &parsing.ErrNoTopLevelSelect{}},
		{name: "drop", query: "drop table foo", expectedErrType: &parsing.ErrNoTopLevelSelect{}},

		// Check no FROM SHARE/UPDATE
		{name: "for share", query: "select * from foo for share", expectedErrType: &parsing.ErrNoForUpdateOrShare{}},
		{name: "for update", query: "select * from foo for update", expectedErrType: &parsing.ErrNoForUpdateOrShare{}},

		// Check no system-tables reference
		{name: "reference system table", query: "select * from system_tables", expectedErrType: &parsing.ErrSystemTableReferencing{}},
		{name: "reference system table with inner join", query: "select * from foo inner join system_tables on a=b", expectedErrType: &parsing.ErrSystemTableReferencing{}},
		{name: "reference system table in nested FROM SELECT", query: "select * from foo inner join (select * from system_tables) j on a=b", expectedErrType: &parsing.ErrSystemTableReferencing{}},
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
				require.IsType(t, tc.expectedErrType, err)
			}
		}(it))
	}
}
