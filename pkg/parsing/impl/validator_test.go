package impl_test

import (
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	postgresparser "github.com/textileio/go-tableland/pkg/parsing/impl"
)

func TestRunSQL(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		query       string
		tableID     *big.Int
		namePrefix  string
		isWriteStmt bool
		expErrType  interface{}
	}

	writeQueryTests := []testCase{
		// Malformed queries.
		{
			name:       "malformed insert",
			query:      "insert into foo valuez (1, 1)",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "malformed update",
			query:      "update foo sez a=1, b=2",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "malformed delete",
			query:      "delete fromz foo where a=2",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Invalid table name format.
		{
			name:       "suffix is not an integer",
			query:      "delete from oops_z123 where a=2",
			expErrType: ptr2ErrInvalidTableName(),
		},
		{
			name:       "suffix cannot include 't' even in long names",
			query:      "delete from person_t123 where a=2",
			expErrType: ptr2ErrInvalidTableName(),
		},
		{
			name:       "non-numeric id",
			query:      "delete from person_tWrong where a=2",
			expErrType: ptr2ErrInvalidTableName(),
		},

		// Valid insert and updates.
		{
			name:       "valid insert",
			query:      "insert into duke_3333 values ('hello', 1, 2)",
			tableID:    big.NewInt(3333),
			namePrefix: "duke",
			expErrType: nil,
		},
		{
			name:       "valid simple update without name prefix needing 't' prefix",
			query:      "update t0 set a=1 where b='hello'",
			tableID:    big.NewInt(0),
			namePrefix: "",
			expErrType: nil,
		},
		{
			name:       "valid delete",
			query:      "delete from i_like_border_cases_10 where a=2",
			tableID:    big.NewInt(10),
			namePrefix: "i_like_border_cases",
			expErrType: nil,
		},
		{
			name:       "valid custom func call",
			query:      "insert into hoop_3 values (myfunc(1))",
			tableID:    big.NewInt(3),
			namePrefix: "hoop",
			expErrType: nil,
		},
		{
			name:       "multi statement",
			query:      "update a_10 set a=1; update a_10 set b=1;",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: nil,
		},

		// Only reference a single table
		{
			name:       "update different tables",
			query:      "update foo set a=1;update bar set a=2",
			expErrType: ptr2ErrMultiTableReference(),
		},

		// Empty statement.
		{
			name:       "no statements",
			query:      "",
			expErrType: ptr2ErrEmptyStatement(),
		},

		// Check not allowed top-statements.
		{
			name:       "create",
			query:      "create table foo (bar int)",
			expErrType: ptr2ErrNoTopLevelUpdateInsertDelete(),
		},
		{
			name:       "drop",
			query:      "drop table foo",
			expErrType: ptr2ErrNoTopLevelUpdateInsertDelete(),
		},
		{
			name:       "update select",
			query:      "update foo set a=1;select * from foo",
			expErrType: ptr2ErrNoTopLevelUpdateInsertDelete(),
		},

		// Disallow JOINs and sub-queries
		{
			name:       "insert subquery",
			query:      "insert into foo select * from bar",
			expErrType: ptr2ErrJoinOrSubquery(),
		},
		{
			name:       "update join",
			query:      "update foo set a=1 from bar",
			expErrType: ptr2ErrJoinOrSubquery(),
		},
		{
			name:       "update where subquery",
			query:      "update foo set a=1 where a=(select a from bar limit 1) and b=1",
			expErrType: ptr2ErrJoinOrSubquery(),
		},
		{
			name:       "delete where subquery",
			query:      "delete from foo where a=(select a from bar limit 1)",
			expErrType: ptr2ErrJoinOrSubquery(),
		},

		// Disallow RETURNING clauses
		{
			name:  "update returning",
			query: "update foo set a=a+1 returning a", expErrType: ptr2ErrReturningClause(),
		},
		{
			name:       "insert returning",
			query:      "insert into foo values (1, 'bar') returning a",
			expErrType: ptr2ErrReturningClause(),
		},
		{
			name:       "delete returning",
			query:      "delete from foo where a=1 returning b",
			expErrType: ptr2ErrReturningClause(),
		},

		// Check no system-tables references.
		{
			name:       "update system table",
			query:      "update system_tables set a=1",
			expErrType: ptr2ErrSystemTableReferencing(),
		},
		{
			name:       "insert system table",
			query:      "insert into system_tables values ('foo')",
			expErrType: ptr2ErrSystemTableReferencing(),
		},
		{
			name:       "delete system table",
			query:      "delete from system_tables",
			expErrType: ptr2ErrSystemTableReferencing(),
		},

		// Check non-deterministic functions.
		{
			name:       "insert current_timestamp lower",
			query:      "insert into foo values (current_timestamp, 'lolz')",
			expErrType: ptr2ErrNonDeterministicFunction(),
		},
		{
			name:       "insert current_timestamp case-insensitive",
			query:      "insert into foo values (current_TiMeSTamP, 'lolz')",
			expErrType: ptr2ErrNonDeterministicFunction(),
		},
		{
			name:       "update set current_timestamp",
			query:      "update foo set a=current_timestamp, b=2",
			expErrType: ptr2ErrNonDeterministicFunction(),
		},
		{
			name:       "update where current_timestamp",
			query:      "update foo set a=1 where b=current_timestamp",
			expErrType: ptr2ErrNonDeterministicFunction(),
		},
		{
			name:       "delete where current_timestamp",
			query:      "delete from foo where a=current_timestamp",
			expErrType: ptr2ErrNonDeterministicFunction(),
		},
	}
	for i := range writeQueryTests {
		writeQueryTests[i].isWriteStmt = true
	}

	readQueryTests := []testCase{
		// Malformed query.
		{
			name:       "malformed query",
			query:      "shelect * from foo",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Valid read-queries.
		{
			name:       "valid all",
			query:      "select * from t1234",
			tableID:    big.NewInt(1234),
			namePrefix: "",
			expErrType: nil,
		},
		{
			name:       "valid defined rows",
			query:      "select row1, row2 from zoo_4321 where a=b",
			tableID:    big.NewInt(4321),
			namePrefix: "zoo",
			expErrType: nil,
		},

		// No JOINs
		{
			name:       "with join",
			query:      "select * from foo inner join bar on a=b",
			expErrType: ptr2ErrJoinOrSubquery(),
		},
		{
			name:       "with subselect",
			query:      "select * from foo where a in (select b from zoo)",
			expErrType: ptr2ErrJoinOrSubquery(),
		},
		{
			name:       "column with subquery",
			query:      "select (select * from bar limit 1) from foo",
			expErrType: ptr2ErrJoinOrSubquery(),
		},

		// Single-statement check.
		{
			name:       "single statement fail",
			query:      "select * from foo; select * from bar",
			expErrType: ptr2ErrNoSingleStatement(),
		},
		{
			name:       "no statements",
			query:      "",
			expErrType: ptr2ErrEmptyStatement(),
		},

		// Check no FROM SHARE/UPDATE
		{
			name:       "for share",
			query:      "select * from foo for share",
			expErrType: ptr2ErrNoForUpdateOrShare(),
		},
		{
			name:       "for update",
			query:      "select * from foo for update",
			expErrType: ptr2ErrNoForUpdateOrShare(),
		},
	}

	tests := append(readQueryTests, writeQueryTests...)

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New("system_")
				rs, wss, err := parser.ValidateRunSQL(tc.query)
				if tc.expErrType == nil {
					require.NoError(t, err)

					if tc.isWriteStmt {
						require.NotEmpty(t, wss)
						for _, ws := range wss {
							require.Equal(t, tc.tableID.String(), ws.GetTableID().String())
							require.Equal(t, tc.namePrefix, ws.GetNamePrefix())
						}
					} else {
						require.NotNil(t, rs)
						require.Equal(t, tc.tableID.String(), rs.GetTableID().String())
						require.Equal(t, tc.namePrefix, rs.GetNamePrefix())
					}
					return
				}
				require.ErrorAs(t, err, tc.expErrType)
			}
		}(it))
	}
}

func TestCreateTableChecks(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		query      string
		expErrType interface{}
	}
	tests := []testCase{
		// Malformed query.
		{
			name:       "malformed query",
			query:      "create tablez foo (foo int)",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Single-statement check.
		{
			name:       "two creates",
			query:      "create table foo (a int); create table bar (a int);",
			expErrType: ptr2ErrNoSingleStatement(),
		},
		{
			name:       "no statements",
			query:      "",
			expErrType: ptr2ErrEmptyStatement(),
		},

		// Check CREATE OF semantics.
		{
			name:       "create of",
			query:      "create table foo of other;",
			expErrType: nil,
		},
		{
			name:       "create of constraint",
			query:      "create table foo of other ( primary key (id) );",
			expErrType: nil,
		},

		// Check CREATE with column CONSTRAINTS
		{
			name:       "create with constraint",
			query:      "create table foo ( id int not null, name text );",
			expErrType: nil,
		},
		{
			name:       "create with extra constraint",
			query:      "create table foo ( id int not null, constraint foo_pk primary key (id) );",
			expErrType: nil,
		},

		// Check top-statement is only CREATE.
		{
			name:       "select",
			query:      "select * from foo",
			expErrType: ptr2ErrNoTopLevelCreate(),
		},
		{
			name:       "update",
			query:      "update foo set bar=1",
			expErrType: ptr2ErrNoTopLevelCreate(),
		},
		{
			name:       "insert",
			query:      "insert into foo values (1)",
			expErrType: ptr2ErrNoTopLevelCreate(),
		},
		{
			name:       "drop",
			query:      "drop table foo",
			expErrType: ptr2ErrNoTopLevelCreate(),
		},
		{
			name:       "delete",
			query:      "delete from foo",
			expErrType: ptr2ErrNoTopLevelCreate(),
		},

		// Valid table with all accepted types.
		{
			name: "valid all",
			query: `create table foo (
				   zint  int,
				   zint2 int2,
				   zint4 int4,
				   zserial serial,
				   zint8 int8,
				   zbigserial bigserial,
				   zbigint bigint,
				   zsmallint smallint,

				   ztext text,
				   zuri uri,
				   zvarchar varchar(10),
				   zbpchar bpchar,
				   zdate date,

				   zbool bool,

				   zfloat4 float4,
				   zfloat8 float8,

				   znumeric numeric,

				   ztimestamp timestamp,
				   ztimestamptz timestamptz,
				   zuuid uuid,

				   zjson json
			       )`,
			expErrType: nil,
		},

		// Tables with invalid columns.
		{
			name:       "xml column",
			query:      "create table foo (foo xml)",
			expErrType: ptr2ErrInvalidColumnType(),
		},
		{
			name:       "money column",
			query:      "create table foo (foo money)",
			expErrType: ptr2ErrInvalidColumnType(),
		},
		{
			name:       "polygon column",
			query:      "create table foo (foo polygon)",
			expErrType: ptr2ErrInvalidColumnType(),
		},
		{
			name:       "jsonb column",
			query:      "create table foo (foo jsonb)",
			expErrType: ptr2ErrInvalidColumnType(),
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New("system_")
				_, err := parser.ValidateCreateTable(tc.query)
				if tc.expErrType == nil {
					require.NoError(t, err)
					return
				}
				require.ErrorAs(t, err, tc.expErrType)
			}
		}(it))
	}
}

func TestCreateTableResult(t *testing.T) {
	t.Parallel()

	type rawQueryTableID struct {
		id       int64
		rawQuery string
	}

	type testCase struct {
		name             string
		query            string
		expNamePrefix    string
		expStructureHash string

		expRawQueries []rawQueryTableID
	}
	tests := []testCase{
		{
			name: "single col",
			query: `create table foo (
				   bar int
			       )`,
			expNamePrefix: "foo",
			// sha256(bar int4)
			expStructureHash: "60b0e90a94273211e4836dc11d8eebd96e8020ce3408dd112ba9c42e762fe3cc",
			expRawQueries: []rawQueryTableID{
				{id: 1, rawQuery: "CREATE TABLE t1 (bar int)"},
				{id: 42, rawQuery: "CREATE TABLE t42 (bar int)"},
				{id: 2929392, rawQuery: "CREATE TABLE t2929392 (bar int)"},
			},
		},
		{
			name: "multiple cols",
			query: `create table person (
				   name text,
				   age int,
				   fav_color varchar(10)
			       )`,
			expNamePrefix: "person",
			// sha256(name:text,age:int4,fav_color:varchar)
			expStructureHash: "3e846cb815f96b1a572246e1bf5eb5eec8a93598aa4a9741e7dade425ff2dc69",
			expRawQueries: []rawQueryTableID{
				{id: 1, rawQuery: "CREATE TABLE t1 (name text, age int, fav_color varchar(10))"},
				{id: 42, rawQuery: "CREATE TABLE t42 (name text, age int, fav_color varchar(10))"},
				{id: 2929392, rawQuery: "CREATE TABLE t2929392 (name text, age int, fav_color varchar(10))"},
			},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New("system_")
				cs, err := parser.ValidateCreateTable(tc.query)
				require.NoError(t, err)

				require.Equal(t, tc.expNamePrefix, cs.GetNamePrefix())
				require.Equal(t, tc.expStructureHash, cs.GetStructureHash())
				for _, erq := range tc.expRawQueries {
					rq, err := cs.GetRawQueryForTableID(tableland.TableID(*big.NewInt(erq.id)))
					require.NoError(t, err)
					require.Equal(t, erq.rawQuery, rq)
				}
			}
		}(it))
	}
}

func TestGetWriteStatements(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name          string
		query         string
		expectedStmts []string
	}
	tests := []testCase{
		{
			name:  "double update",
			query: "update foo_100 set a=1;update foo_100 set b=2;",
			expectedStmts: []string{
				"UPDATE t100 SET a = 1",
				"UPDATE t100 SET b = 2",
			},
		},
		{
			name:  "insert update",
			query: "insert into foo_0 values (1);update foo_0 set b=2;",
			expectedStmts: []string{
				"INSERT INTO t0 VALUES (1)",
				"UPDATE t0 SET b = 2",
			},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := postgresparser.New("system_")
				rs, stmts, err := parser.ValidateRunSQL(tc.query)
				require.NoError(t, err)
				require.Nil(t, rs)

				for i := range stmts {
					desugared, err := stmts[i].GetDesugaredQuery()
					require.NoError(t, err)
					require.Equal(t, tc.expectedStmts[i], desugared)
				}
			}
		}(it))
	}
}

// Helpers to have a pointer to pointer for generic test-case running.
func ptr2ErrInvalidSyntax() **parsing.ErrInvalidSyntax {
	var e *parsing.ErrInvalidSyntax
	return &e
}
func ptr2ErrNoSingleStatement() **parsing.ErrNoSingleStatement {
	var e *parsing.ErrNoSingleStatement
	return &e
}
func ptr2ErrEmptyStatement() **parsing.ErrEmptyStatement {
	var e *parsing.ErrEmptyStatement
	return &e
}
func ptr2ErrNoForUpdateOrShare() **parsing.ErrNoForUpdateOrShare {
	var e *parsing.ErrNoForUpdateOrShare
	return &e
}
func ptr2ErrSystemTableReferencing() **parsing.ErrSystemTableReferencing {
	var e *parsing.ErrSystemTableReferencing
	return &e
}
func ptr2ErrNoTopLevelUpdateInsertDelete() **parsing.ErrNoTopLevelUpdateInsertDelete {
	var e *parsing.ErrNoTopLevelUpdateInsertDelete
	return &e
}
func ptr2ErrReturningClause() **parsing.ErrReturningClause {
	var e *parsing.ErrReturningClause
	return &e
}
func ptr2ErrNonDeterministicFunction() **parsing.ErrNonDeterministicFunction {
	var e *parsing.ErrNonDeterministicFunction
	return &e
}
func ptr2ErrJoinOrSubquery() **parsing.ErrJoinOrSubquery {
	var e *parsing.ErrJoinOrSubquery
	return &e
}
func ptr2ErrNoTopLevelCreate() **parsing.ErrNoTopLevelCreate {
	var e *parsing.ErrNoTopLevelCreate
	return &e
}
func ptr2ErrInvalidColumnType() **parsing.ErrInvalidColumnType {
	var e *parsing.ErrInvalidColumnType
	return &e
}
func ptr2ErrMultiTableReference() **parsing.ErrMultiTableReference {
	var e *parsing.ErrMultiTableReference
	return &e
}
func ptr2ErrInvalidTableName() **parsing.ErrInvalidTableName {
	var e *parsing.ErrInvalidTableName
	return &e
}
