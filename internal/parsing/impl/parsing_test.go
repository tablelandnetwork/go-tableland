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

		// Single-statement check
		{name: "single statement fail", query: "select * from foo; select * from bar", expectedErrType: &parsing.ErrNoSingleStatement{}},
		{name: "no statements", query: "", expectedErrType: &parsing.ErrNoSingleStatement{}},

		// Check top-statement is only SELECT.
		{name: "create", query: "create table foo (bar int)", expectedErrType: &parsing.ErrNoTopLevelSelect{}},
		{name: "update", query: "update table foo set bar=1", expectedErrType: &parsing.ErrNoTopLevelSelect{}},
		{name: "drop", query: "drop table foo", expectedErrType: &parsing.ErrNoTopLevelSelect{}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New()
				err := parser.ValidateReadQuery(tc.query)
				if tc.expectedErrType == nil {
					require.NoError(t, err)
					return
				}
				require.Error(t, err)
				require.ErrorAs(t, err, &tc.expectedErrType)
			}
		}(tc))
	}
}
