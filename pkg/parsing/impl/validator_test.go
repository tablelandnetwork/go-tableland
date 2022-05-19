package impl_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	parser "github.com/textileio/go-tableland/pkg/parsing/impl"
)

func TestReadRunSQL(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name        string
		query       string
		expRawQuery string
		expErrType  interface{}
	}

	tests := []testCase{
		// Malformed query.
		{
			name:       "malformed query",
			query:      "shelect * from foo",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Valid read-queries.
		{
			name:        "valid all",
			query:       "select * from _1234",
			expRawQuery: "SELECT * FROM _1337_1234",
			expErrType:  nil,
		},
		{
			name:        "valid defined rows",
			query:       "select row1, row2 from zoo_4321 where a = b",
			expRawQuery: "SELECT row1, row2 FROM _1337_4321 WHERE a = b",
			expErrType:  nil,
		},

		// Allow joins and sub-queries
		{
			name:        "with join",
			query:       "select * from foo_1 join bar_2 on a=b",
			expRawQuery: "SELECT * FROM _1337_1 JOIN _1337_2 ON a = b",
			expErrType:  nil,
		},
		{
			name:        "with subselect",
			query:       "select * from foo_1 where a in (select b from zoo_5)",
			expRawQuery: "SELECT * FROM _1337_1 WHERE a IN (SELECT b FROM _1337_5)",
			expErrType:  nil,
		},
		{
			name:        "column with subquery",
			query:       "select (select * from bar_2 limit 1) from foo_3",
			expRawQuery: "SELECT (SELECT * FROM _1337_2 LIMIT 1) FROM _1337_3",
			expErrType:  nil,
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

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
				rs, _, err := parser.ValidateRunSQL(tc.query)

				if tc.expErrType == nil {
					require.NoError(t, err)
					require.NotNil(t, rs)
					desugared, err := rs.GetDesugaredQuery()
					require.NoError(t, err)
					require.Equal(t, tc.expRawQuery, desugared)
					return
				}
				require.ErrorAs(t, err, tc.expErrType)
			}
		}(it))
	}
}

func TestMutatingRunSQL(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		query      string
		tableID    *big.Int
		namePrefix string
		expErrType interface{}
	}

	writeQueryTests := []testCase{
		// Malformed queries.
		{
			name:       "malformed insert",
			query:      "insert into foo valuez (1, 1)",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "numeric tablename",
			query:      "insert into 10 valuez (1, 1)",
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
			name:       "valid simple update without name prefix needing '_' prefix",
			query:      "update _0 set a=1 where b='hello'",
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
			expErrType: ptr2ErrStatementIsNotSupported(),
		},
		{
			name:       "drop",
			query:      "drop table foo",
			expErrType: ptr2ErrStatementIsNotSupported(),
		},
		{
			name:       "update select",
			query:      "update foo set a=1;select * from foo",
			expErrType: ptr2ErrStatementIsNotSupported(),
		},

		// Allow JOINs and sub-queries
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

		// Disallow alias on relation
		{
			name:       "update alias",
			query:      "update foo as f set f.a=f.a+1",
			expErrType: ptr2ErrRelationAlias(),
		},
		{
			name:       "insert alias",
			query:      "insert into foo as f values (1, 'bar')",
			expErrType: ptr2ErrRelationAlias(),
		},
		{
			name:       "delete alias",
			query:      "delete from foo as f where f.a=1",
			expErrType: ptr2ErrRelationAlias(),
		},

		// Check no system-tables references.
		{
			name:       "update system table",
			query:      "update registry set a=1",
			expErrType: ptr2ErrSystemTableReferencing(),
		},
		{
			name:       "insert system table",
			query:      "insert into registry values ('foo')",
			expErrType: ptr2ErrSystemTableReferencing(),
		},
		{
			name:       "delete system table",
			query:      "delete from registry",
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

	grantQueryTests := []testCase{
		// Valid grant statement
		{
			name:       "grant statement",
			query:      "grant insert, update, delete on a_10 to \"0xd43c59d5694ec111eb9e986c233200b14249558d\",  \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\"", //nolint
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: nil,
		},
		{
			name:       "revoke statement",
			query:      "revoke insert, update, delete on a_10 from \"0xd43c59d5694ec111eb9e986c233200b14249558d\",  \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\"", //nolint
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: nil,
		},

		// grant membership is not supported
		{
			name:       "grant membership",
			query:      "GRANT admin to joe;",
			expErrType: ptr2ErrStatementIsNotSupported(),
		},

		// disallow privileges
		{
			name:       "grant statement all privileges",
			query:      "grant all privileges on a_10 to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrAllPrivilegesNotAllowed(),
		},
		{
			name:       "revoke statement all privileges",
			query:      "revoke all on a_10 from role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrAllPrivilegesNotAllowed(),
		},

		{
			name:       "grant statement connect",
			query:      "grant connect on a_10 to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrNoInsertUpdateDeletePrivilege(),
		},

		// disallow grant on multiple objects
		{
			name:       "grant statement multiple table",
			query:      "grant insert, update, delete on a_10, a_11 to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrNoSingleTableReference(),
		},
		{
			name:       "revoke statement multiple table",
			query:      "revoke insert, update, delete on a_10, a_11 from role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrNoSingleTableReference(),
		},
		// disallow grant on target object different than ACL_TARGET_OBJECT
		{
			name:       "grant statement all tables",
			query:      "grant insert, update, delete on all tables in schema s to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrTargetTypeIsNotObject(),
		},
		{
			name:       "revoke statement all tables",
			query:      "revoke insert, update, delete on all tables in schema s from role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrTargetTypeIsNotObject(),
		},

		// disallow grant on object that is not table
		{
			name:       "grant statement database",
			query:      "grant insert, update, delete on database db to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement sequence",
			query:      "grant insert, update, delete on sequence s to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement schema",
			query:      "grant insert, update, delete on schema s to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement fdw",
			query:      "grant insert, update, delete on foreign data wrapper fdw to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement foreign server",
			query:      "grant insert, update, delete on foreign server fdw to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement function",
			query:      "grant insert, update, delete on function f to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement language",
			query:      "grant insert, update, delete on language l to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement large object",
			query:      "grant insert, update, delete on large object 1 to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},
		{
			name:       "grant statement tablespace",
			query:      "grant insert, update, delete on tablespace tblsp to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrObjectTypeIsNotTable(),
		},

		// disallow grant on roles that are not cstring
		{
			name:       "grant statement public",
			query:      "grant insert, update, delete on a_10 to public",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrRoleIsNotCString(),
		},
		{
			name:       "revoke statement public",
			query:      "revoke insert, update, delete on a_10 from public",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrRoleIsNotCString(),
		},

		// disallow grant on roles that are not eth addresses
		{
			name:       "grant statement eth address",
			query:      "grant insert, update, delete on a_10 to role",
			tableID:    big.NewInt(10),
			namePrefix: "a",
			expErrType: ptr2ErrRoleIsNotAnEthAddress(),
		},
	}

	tests := append(writeQueryTests, grantQueryTests...)
	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
				_, mss, err := parser.ValidateRunSQL(tc.query)

				if tc.expErrType == nil {
					require.NoError(t, err)

					require.NotEmpty(t, mss)
					for _, ms := range mss {
						require.Equal(t, tc.tableID.String(), ms.GetTableID().String())
						require.Equal(t, tc.namePrefix, ms.GetNamePrefix())
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
				   zuri text,
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
				parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
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
				{id: 1, rawQuery: "CREATE TABLE _1337_1 (bar int)"},
				{id: 42, rawQuery: "CREATE TABLE _1337_42 (bar int)"},
				{id: 2929392, rawQuery: "CREATE TABLE _1337_2929392 (bar int)"},
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
				{id: 1, rawQuery: "CREATE TABLE _1337_1 (name text, age int, fav_color varchar(10))"},
				{id: 42, rawQuery: "CREATE TABLE _1337_42 (name text, age int, fav_color varchar(10))"},
				{id: 2929392, rawQuery: "CREATE TABLE _1337_2929392 (name text, age int, fav_color varchar(10))"},
			},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
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

func TestCreateTableColLimit(t *testing.T) {
	t.Parallel()

	maxAllowedColumns := 3
	parser := parser.New([]string{"system_", "registry"}, 1337, maxAllowedColumns, 0)

	t.Run("success one column", func(t *testing.T) {
		_, err := parser.ValidateCreateTable("create table foo (a int)")
		require.NoError(t, err)
	})
	t.Run("success exact max columns", func(t *testing.T) {
		_, err := parser.ValidateCreateTable("create table foo (a int, b text,c int)")
		require.NoError(t, err)
	})
	t.Run("failure max columns exceeded", func(t *testing.T) {
		_, err := parser.ValidateCreateTable("create table foo (a int, b text,c int, d int)")
		var expErr *parsing.ErrTooManyColumns
		require.ErrorAs(t, err, &expErr)
		require.Equal(t, 4, expErr.ColumnCount)
		require.Equal(t, maxAllowedColumns, expErr.MaxAllowed)
	})
}

func TestCreateTableTextLength(t *testing.T) {
	t.Parallel()

	textMaxLength := 4
	parser := parser.New([]string{"system_", "registry"}, 1337, 0, textMaxLength)

	t.Run("success half limit", func(t *testing.T) {
		_, _, err := parser.ValidateRunSQL(`insert into _0 values ('aa')`)
		require.NoError(t, err)
	})
	t.Run("success exact max length", func(t *testing.T) {
		_, _, err := parser.ValidateRunSQL(`insert into _0 values ('aaaa')`)
		require.NoError(t, err)
	})
	t.Run("failure insert max length exceeded", func(t *testing.T) {
		_, _, err := parser.ValidateRunSQL(`insert into _0 values ('aaaaa')`)
		var expErr *parsing.ErrTextTooLong
		require.ErrorAs(t, err, &expErr)
		require.Equal(t, 5, expErr.Length)
		require.Equal(t, textMaxLength, expErr.MaxAllowed)
	})
	t.Run("failure update max length exceeded", func(t *testing.T) {
		_, _, err := parser.ValidateRunSQL(`update _0 set a='aaaaa'`)
		var expErr *parsing.ErrTextTooLong
		require.ErrorAs(t, err, &expErr)
		require.Equal(t, 5, expErr.Length)
		require.Equal(t, textMaxLength, expErr.MaxAllowed)
	})
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
				"UPDATE _1337_100 SET a = 1",
				"UPDATE _1337_100 SET b = 2",
			},
		},
		{
			name:  "insert update",
			query: "insert into foo_0 values (1);update foo_0 set b=2;",
			expectedStmts: []string{
				"INSERT INTO _1337_0 VALUES (1)",
				"UPDATE _1337_0 SET b = 2",
			},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
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

func TestGetGrantStatementRolesAndPrivileges(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name         string
		query        string
		roles        []common.Address
		privileges   tableland.Privileges
		expectedStmt string
	}
	tests := []testCase{
		{
			name:         "grant",
			query:        "grant insert, UPDATE on a_100 to \"0xd43c59d5694ec111eb9e986c233200b14249558d\";",
			roles:        []common.Address{common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")},
			privileges:   []tableland.Privilege{tableland.PrivInsert, tableland.PrivUpdate},
			expectedStmt: "GRANT insert, update ON _1337_100 TO \"0xd43c59d5694ec111eb9e986c233200b14249558d\"",
		},

		{
			name:  "revoke",
			query: "revoke delete on a_100 from \"0xd43c59d5694ec111eb9e986c233200b14249558d\", \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\";", // nolint
			roles: []common.Address{
				common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d"),
				common.HexToAddress("0x4afe8e30db4549384b0a05bb796468b130c7d6e0"),
			},
			privileges:   []tableland.Privilege{tableland.PrivDelete},
			expectedStmt: "REVOKE delete ON _1337_100 FROM \"0xd43c59d5694ec111eb9e986c233200b14249558d\", \"0x4afe8e30db4549384b0a05bb796468b130c7d6e0\"", // nolint
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
				rs, stmts, err := parser.ValidateRunSQL(tc.query)
				require.NoError(t, err)
				require.Nil(t, rs)

				for i := range stmts {
					gs, ok := stmts[i].(parsing.SugaredGrantStmt)
					require.True(t, ok)
					desugared, err := gs.GetDesugaredQuery()
					require.NoError(t, err)
					require.Equal(t, tc.expectedStmt, desugared)
					require.Equal(t, tc.roles, gs.GetRoles())
					require.Equal(t, tc.privileges, gs.GetPrivileges())
				}
			}
		}(it))
	}
}

func TestWriteStatementAddWhereClause(t *testing.T) {
	t.Parallel()

	testCase := []struct {
		name        string
		query       string
		whereClause string
		expQuery    string
	}{
		{
			name:        "no-where-clause",
			query:       "UPDATE foo_1337 SET id = 1",
			whereClause: "bar = 1",
			expQuery:    "UPDATE _1337_1337 SET id = 1 WHERE bar = 1",
		},
		{
			name:        "with-where-clause",
			query:       "UPDATE foo_1337 SET id = 1 WHERE bar = 1",
			whereClause: "c in (1, 2)",
			expQuery:    "UPDATE _1337_1337 SET id = 1 WHERE bar = 1 AND c IN (1, 2)",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
			rs, mss, err := parser.ValidateRunSQL(tc.query)
			require.NoError(t, err)
			require.Nil(t, rs)
			require.Len(t, mss, 1)

			ws, ok := mss[0].(parsing.SugaredWriteStmt)
			require.True(t, ok)

			err = ws.AddWhereClause(tc.whereClause)
			require.NoError(t, err)

			sql, err := ws.GetDesugaredQuery()
			require.NoError(t, err)
			require.Equal(t, tc.expQuery, sql)
		})
	}
}

func TestWriteStatementAddReturningClause(t *testing.T) {
	t.Parallel()
	t.Run("insert-add-returning", func(t *testing.T) {
		t.Parallel()

		parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
		rs, mss, err := parser.ValidateRunSQL("insert into foo_0 VALUES ('bar')")
		require.NoError(t, err)
		require.Nil(t, rs)
		require.Len(t, mss, 1)

		ws, ok := mss[0].(parsing.SugaredWriteStmt)
		require.True(t, ok)

		err = ws.AddReturningClause()
		require.NoError(t, err)

		sql, err := ws.GetDesugaredQuery()
		require.NoError(t, err)
		require.Equal(t, "INSERT INTO _1337_0 VALUES ('bar') RETURNING ctid", sql)
	})

	t.Run("update-add-returning", func(t *testing.T) {
		t.Parallel()

		parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
		rs, mss, err := parser.ValidateRunSQL("update foo_0 set foo = 'bar'")
		require.NoError(t, err)
		require.Nil(t, rs)
		require.Len(t, mss, 1)

		ws, ok := mss[0].(parsing.SugaredWriteStmt)
		require.True(t, ok)

		err = ws.AddReturningClause()
		require.NoError(t, err)

		sql, err := ws.GetDesugaredQuery()
		require.NoError(t, err)
		require.Equal(t, "UPDATE _1337_0 SET foo = 'bar' RETURNING ctid", sql)
	})

	t.Run("delete-add-returning-error", func(t *testing.T) {
		t.Parallel()

		parser := parser.New([]string{"system_", "registry"}, 1337, 0, 0)
		rs, mss, err := parser.ValidateRunSQL("DELETE FROM foo_0 WHERE foo = 'bar'")
		require.NoError(t, err)
		require.Nil(t, rs)
		require.Len(t, mss, 1)

		ws, ok := mss[0].(parsing.SugaredWriteStmt)
		require.True(t, ok)

		err = ws.AddReturningClause()
		require.ErrorAs(t, err, &parsing.ErrCantAddReturningOnDELETE)
	})
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

func ptr2ErrReturningClause() **parsing.ErrReturningClause {
	var e *parsing.ErrReturningClause
	return &e
}

func ptr2ErrRelationAlias() **parsing.ErrRelationAlias {
	var e *parsing.ErrRelationAlias
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

func ptr2ErrNoSingleTableReference() **parsing.ErrNoSingleTableReference {
	var e *parsing.ErrNoSingleTableReference
	return &e
}

func ptr2ErrObjectTypeIsNotTable() **parsing.ErrObjectTypeIsNotTable {
	var e *parsing.ErrObjectTypeIsNotTable
	return &e
}

func ptr2ErrRoleIsNotCString() **parsing.ErrRoleIsNotCString {
	var e *parsing.ErrRoleIsNotCString
	return &e
}

func ptr2ErrTargetTypeIsNotObject() **parsing.ErrTargetTypeIsNotObject {
	var e *parsing.ErrTargetTypeIsNotObject
	return &e
}

func ptr2ErrAllPrivilegesNotAllowed() **parsing.ErrAllPrivilegesNotAllowed {
	var e *parsing.ErrAllPrivilegesNotAllowed
	return &e
}

func ptr2ErrNoInsertUpdateDeletePrivilege() **parsing.ErrNoInsertUpdateDeletePrivilege {
	var e *parsing.ErrNoInsertUpdateDeletePrivilege
	return &e
}

func ptr2ErrStatementIsNotSupported() **parsing.ErrStatementIsNotSupported {
	var e *parsing.ErrStatementIsNotSupported
	return &e
}

func ptr2ErrRoleIsNotAnEthAddress() **parsing.ErrRoleIsNotAnEthAddress {
	var e *parsing.ErrRoleIsNotAnEthAddress
	return &e
}
