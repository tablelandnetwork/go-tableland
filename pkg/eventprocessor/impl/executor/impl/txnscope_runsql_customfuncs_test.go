package impl

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCustomFunctionsWriteQuery(t *testing.T) {
	t.Parallel()
	type subTest struct {
		name                 string
		query                string
		mustFail             bool
		newExecutorWithTable func() (*Executor, string)
		assertExpectation    func(dbURI string, txnHash string)
	}

	checkTxnHash := func(dbURI string, txnHash string) {
		require.Equal(t, txnHash, tableReadString(t, dbURI, "select * from foo_1337_100"))
	}
	checkBlockNumberEq1 := func(dbURI string, txnHash string) {
		require.Equal(t, 1, tableReadInteger(t, dbURI, "select * from foo_1337_100"))
	}
	newExecutorWithIntegerTable := func() (*Executor, string) { return newExecutorWithIntegerTable(t, 0) }
	newExecutorWithStringTable := func() (*Executor, string) { return newExecutorWithStringTable(t, 0) }

	subTests := []subTest{
		// txn_hash()
		{
			name:                 "txn_hash() lowercase",
			query:                "insert into foo_1337_100 values (txn_hash())",
			newExecutorWithTable: newExecutorWithStringTable,
			assertExpectation:    checkTxnHash,
		},
		{
			name:                 "txn_hash() mixed case",
			query:                "insert into foo_1337_100 values (TxN_HaSh())",
			newExecutorWithTable: newExecutorWithStringTable,
			assertExpectation:    checkTxnHash,
		},
		{
			name:                 "txn_hash() upper case",
			query:                "insert into foo_1337_100 values (TXN_HASH())",
			newExecutorWithTable: newExecutorWithStringTable,
			assertExpectation:    checkTxnHash,
		},
		{
			name:                 "txn_hash() with integer argument",
			query:                "insert into foo_1337_100 values (txn_hash(1))",
			newExecutorWithTable: newExecutorWithStringTable,
			mustFail:             true,
		},
		{
			name:                 "txn_hash() with string argument",
			query:                "insert into foo_1337_100 values (txn_hash('i must not have arguments'))",
			newExecutorWithTable: newExecutorWithStringTable,
			mustFail:             true,
		},

		// block_num()
		{
			name:                 "block_num() lower case",
			query:                "insert into foo_1337_100 values (block_num())",
			newExecutorWithTable: newExecutorWithIntegerTable,
			assertExpectation:    checkBlockNumberEq1,
		},
		{
			name:                 "block_num() mixed case",
			query:                "insert into foo_1337_100 values (BlOcK_nUm())",
			newExecutorWithTable: newExecutorWithIntegerTable,
			assertExpectation:    checkBlockNumberEq1,
		},
		{
			name:                 "block_num() upper case",
			query:                "insert into foo_1337_100 values (BLOCK_NUM())",
			newExecutorWithTable: newExecutorWithIntegerTable,
			assertExpectation:    checkBlockNumberEq1,
		},
		{
			// block_num(<chain-id>) must be valid **only** for read queries.
			name:                 "block_num() with integer argument",
			query:                "insert into foo_1337_100 values (block_num(1337))",
			newExecutorWithTable: newExecutorWithIntegerTable,
			mustFail:             true,
		},
		{
			name:                 "block_num() with string argument",
			query:                "insert into foo_1337_100 values (block_num('nope'))",
			newExecutorWithTable: newExecutorWithIntegerTable,
			mustFail:             true,
		},
	}

	for _, test := range subTests {
		t.Run(test.name, func(test subTest) func(t *testing.T) {
			return func(t *testing.T) {
				ctx := context.Background()
				ex, dbURI := test.newExecutorWithTable()

				bs, err := ex.NewBlockScope(ctx, 0)
				require.NoError(t, err)

				txnHash, res, err := execTxnWithRunSQLEvents(t, bs, []string{test.query})
				if test.mustFail {
					require.Error(t, err)
				}
				require.NoError(t, err)
				require.NotNil(t, res.TableID)
				require.Equal(t, int64(100), res.TableID.ToBigInt().Int64())

				require.NoError(t, bs.Commit())
				require.NoError(t, bs.Close())
				require.NoError(t, ex.Close(ctx))

				require.Equal(t, 1, tableReadInteger(t, dbURI, "select count(*) from foo_1337_100"))

				test.assertExpectation(dbURI, txnHash.Hex())
			}
		}(test))
	}
}
