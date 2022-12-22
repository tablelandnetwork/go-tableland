package impl_test

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	parser "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tables"
)

func TestReadRunSQL(t *testing.T) {
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
			query:      "shelect * from foo",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Valid read-queries.
		{
			name:       "valid all",
			query:      "select * from Hello_4_1234",
			expErrType: nil,
		},
		{
			name:       "valid defined rows",
			query:      "select row1, row2 from zoo_4321 where a = b",
			expErrType: nil,
		},

		// Allow joins and sub-queries
		{
			name:       "with join",
			query:      "select * from foo_1 join bar_2 on a=b",
			expErrType: nil,
		},
		{
			name:       "with subselect",
			query:      "select * from foo_1 where a in (select b from zoo_5)",
			expErrType: nil,
		},
		{
			name:       "column with subquery",
			query:      "select (select * from bar_2 limit 1) from foo_3",
			expErrType: nil,
		},
		{
			name: "select with complex subqueries in function arguments",
			query: `select
				json_object(
				  'name', '#' || rigs.id,
				  'external_url', 'https://rigs.tableland.xyz/' || rigs.id,
				  'attributes', json_array(
					  json_object('trait_type', 'Fleet', 'value', rigs.fleet),
					  json_object('trait_type', 'Chassis', 'value', rigs.chassis),
					  json_object('trait_type', 'Wheels', 'value', rigs.wheels),
					  json_object('trait_type', 'Background', 'value', rigs.background),
					  json_object('trait_type', (
						select name from badges_2 where badges.rig_id = rigs.id and position = 1 limit 1
					  ), 'value', (
						select image from badges_2 where badges.rig_id = rigs.id and position = 1 limit 1
					  ))
				  )
			  )
			  from rigs_1;`,
			expErrType: nil,
		},

		// Single-statement check.
		{
			name:       "single statement fail",
			query:      "select * from foo; select * from bar",
			expErrType: ptr2ErrInvalidSyntax(),
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
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "for update",
			query:      "select * from foo for update",
			expErrType: ptr2ErrInvalidSyntax(),
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				parser := newParser(t, []string{"system_", "registry"})
				rs, err := parser.ValidateReadQuery(tc.query)

				if tc.expErrType == nil {
					require.NoError(t, err)
					require.NotNil(t, rs)
					q, err := rs.GetQuery(nil)
					require.NoError(t, err)
					require.Equal(t, tc.query, q)
					return
				}
				require.ErrorAs(t, err, tc.expErrType)
			}
		}(it))
	}
}

func TestWriteQuery(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		query      string
		tableID    *big.Int
		chainID    tableland.ChainID
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
			name:       "table id or chain id is missing",
			query:      "delete from Hello_4 where a=2",
			expErrType: ptr2ErrWrongFormatTableName(),
		},
		{
			name:       "unprefixed table is missing underscore",
			query:      "delete from 4_10 where a=2",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "non-numeric table id",
			query:      "delete from person_4_wrong where a=2",
			expErrType: ptr2ErrWrongFormatTableName(),
		},
		{
			name:       "non-numeric chain id",
			query:      "delete from _wrong_4 where a=2",
			expErrType: ptr2ErrWrongFormatTableName(),
		},

		// Valid insert and updates.
		{
			name:       "valid insert with prefix",
			query:      "insert into duke_4_3333 values ('hello', 1, 2)",
			tableID:    big.NewInt(3333),
			chainID:    4,
			namePrefix: "duke",
			expErrType: nil,
		},
		{
			name:       "valid insert without prefix",
			query:      "insert into _4_3333 values ('hello', 1, 2)",
			tableID:    big.NewInt(3333),
			chainID:    4,
			namePrefix: "",
			expErrType: nil,
		},
		{
			name:       "prefix with multiple underscores",
			query:      "delete from i_like_border_cases_4_10 where a=2",
			tableID:    big.NewInt(10),
			chainID:    4,
			namePrefix: "i_like_border_cases",
			expErrType: nil,
		},
		{
			name:       "prefix with multiple underscores and numbers",
			query:      "delete from i_like_100_border_cases_4_10 where a=2",
			tableID:    big.NewInt(10),
			chainID:    4,
			namePrefix: "i_like_100_border_cases",
			expErrType: nil,
		},
		{
			name:       "valid custom func call",
			query:      "insert into hoop_69_3 values (count(1))",
			tableID:    big.NewInt(3),
			chainID:    69,
			namePrefix: "hoop",
			expErrType: nil,
		},
		{
			name:       "multi statement",
			query:      "update a_4_10 set a=1; update a_4_10 set b=1;",
			tableID:    big.NewInt(10),
			chainID:    4,
			namePrefix: "a",
			expErrType: nil,
		},

		// Only reference a single table
		{
			name:       "update different tables",
			query:      "update foo_4_10 set a=1;update bar_4_12 set a=2",
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
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "update select",
			query:      "update foo set a=1;select * from foo",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// insert with select chain mismatch
		{
			name:       "insert subquery chain mismatch",
			query:      "insert into foo_1_1 select * from bar_2_1",
			expErrType: ptr2ErrInsertWithSelectChainMistmatch(),
		},

		// Disallow JOINs and sub-queries
		{
			name:       "update join",
			query:      "update foo set a=1 from bar",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "update where subquery",
			query:      "update foo set a=1 where a=(select a from bar limit 1) and b=1",
			expErrType: ptr2ErrSubquery(),
		},
		{
			name:       "delete where subquery",
			query:      "delete from foo where a=(select a from bar limit 1)",
			expErrType: ptr2ErrSubquery(),
		},

		// Disallow RETURNING clauses
		{
			name:       "update returning",
			query:      "update foo set a=a+1 returning a",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "insert returning",
			query:      "insert into foo values (1, 'bar') returning a",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "delete returning",
			query:      "delete from foo where a=1 returning b",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Disallow alias on relation
		{
			name:       "update alias",
			query:      "update foo as f set f.a=f.a+1",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "insert alias",
			query:      "insert into foo as f values (1, 'bar')",
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "delete alias",
			query:      "delete from foo as f where f.a=1",
			expErrType: ptr2ErrInvalidSyntax(),
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
			query:      "grant insert, update, delete on a_5_10 to '0xd43c59d5694ec111eb9e986c233200b14249558d',  '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'", // nolint
			tableID:    big.NewInt(10),
			chainID:    5,
			namePrefix: "a",
			expErrType: nil,
		},
		{
			name:       "revoke statement",
			query:      "revoke insert, update, delete on a_8_10 from '0xd43c59d5694ec111eb9e986c233200b14249558d',  '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'", // nolint
			tableID:    big.NewInt(10),
			chainID:    8,
			namePrefix: "a",
			expErrType: nil,
		},

		// disallow grant on roles that are not eth addresses
		{
			name:       "grant statement eth address",
			query:      "grant insert, update, delete on a_10 to 'role'",
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

				parser := newParser(t, []string{"system_", "registry"})
				mss, err := parser.ValidateMutatingQuery(tc.query, tc.chainID)

				if tc.expErrType == nil {
					require.NoError(t, err)

					require.NotEmpty(t, mss)
					for _, ms := range mss {
						require.Equal(t, tc.tableID.String(), ms.GetTableID().String())
						require.Equal(t, tc.namePrefix, ms.GetPrefix())
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
		chainID    tableland.ChainID
		expErrType interface{}
	}
	tests := []testCase{
		// Malformed query.
		{
			name:       "malformed query",
			query:      "create tablez foo (foo int)",
			expErrType: ptr2ErrInvalidSyntax(),
		},

		// Wrong chain id reference
		{
			name:       "wrong chain id without prefix",
			query:      "create table _69 (foo int)",
			chainID:    68,
			expErrType: ptr2ErrInvalidTableName(),
		},
		{
			name:       "wrong chain id with prefix",
			query:      "create table i_am_a_prefix_69 (foo int)",
			chainID:    68,
			expErrType: ptr2ErrInvalidTableName(),
		},
		{
			name:       "missing chain id",
			query:      "create table Hello (foo int)",
			chainID:    68,
			expErrType: ptr2ErrWrongFormatTableName(),
		},
		{
			name:       "missing underscore",
			query:      "create table 69 (foo int)",
			chainID:    69,
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "prefix starting with a number",
			query:      "create table 0Hello_69 (foo int)",
			chainID:    69,
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "prefix starts with sqlite_",
			query:      "create table sqlite_test_69 (foo int)",
			chainID:    69,
			expErrType: ptr2ErrPrefixTableName(),
		},
		{
			name:       "prefix starts with system_",
			query:      "create table system_test_69 (foo int)",
			chainID:    69,
			expErrType: ptr2ErrPrefixTableName(),
		},
		{
			name:       "prefix starts with registry",
			query:      "create table registry_69 (foo int)",
			chainID:    69,
			expErrType: ptr2ErrPrefixTableName(),
		},

		// Single-statement check.
		{
			name:       "two creates",
			query:      "create table foo_4 (a int); create table bar_4 (a int);",
			chainID:    4,
			expErrType: ptr2ErrInvalidSyntax(),
		},
		{
			name:       "no statements",
			query:      "",
			expErrType: ptr2ErrEmptyStatement(),
		},

		// Check CREATE with column CONSTRAINTS
		{
			name:       "create with constraint",
			query:      "create table foo_4 ( id int not null, name text );",
			chainID:    4,
			expErrType: nil,
		},
		{
			name:       "create with extra constraint",
			query:      "create table foo_4 ( id int not null, constraint foo_pk primary key (id) );",
			chainID:    4,
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
			name:       "delete",
			query:      "delete from foo",
			expErrType: ptr2ErrNoTopLevelCreate(),
		},

		// reserved keywords
		{
			name:       "keyword references",
			query:      "create table any_1337 (references text);",
			chainID:    1337,
			expErrType: ptr2ErrKeywordIsNotAllowed(),
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := newParser(t, []string{"system_", "registry", "sqlite_"})
				_, err := parser.ValidateCreateTable(tc.query, tc.chainID)
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
		expPrefix        string
		expStructureHash string

		expRawQueries []rawQueryTableID
	}
	tests := []testCase{
		{
			name: "single col with prefix",
			query: `create table my_10_nth_table_1337 (
				   bar int
			       )`,
			expPrefix: "my_10_nth_table",
			// echo -n bar:INT | shasum -a 256
			expStructureHash: "5d70b398f938650871dd0d6d421e8d1d0c89fe9ed6c8a817c97e951186da7172",
			expRawQueries: []rawQueryTableID{
				{id: 1, rawQuery: "create table my_10_nth_table_1337_1 (bar int) strict"},
				{id: 42, rawQuery: "create table my_10_nth_table_1337_42 (bar int) strict"},
				{id: 2929392, rawQuery: "create table my_10_nth_table_1337_2929392 (bar int) strict"},
			},
		},
		{
			name: "single col without prefix",
			query: `create table _1337 (
				   bar int
			       )`,
			expPrefix: "",
			// echo -n bar:INT | shasum -a 256
			expStructureHash: "5d70b398f938650871dd0d6d421e8d1d0c89fe9ed6c8a817c97e951186da7172",
			expRawQueries: []rawQueryTableID{
				{id: 1, rawQuery: "create table _1337_1 (bar int) strict"},
				{id: 42, rawQuery: "create table _1337_42 (bar int) strict"},
			},
		},
		{
			name: "multiple cols",
			query: `create table person_1337 (
				   name text,
				   age int,
				   fav_color TEXT
			       )`,
			expPrefix: "person",
			// echo -n name:TEXT,age:INT,fav_color:TEXT | shasum -a 256
			expStructureHash: "f45023b189891ad781070ac05374d4e7d7ec7ae007cfd836791c36d609ba7ddd",
			expRawQueries: []rawQueryTableID{
				{id: 1, rawQuery: "create table person_1337_1 (name text, age int, fav_color text) strict"},
				{id: 42, rawQuery: "create table person_1337_42 (name text, age int, fav_color text) strict"},
				{id: 2929392, rawQuery: "create table person_1337_2929392 (name text, age int, fav_color text) strict"},
			},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := newParser(t, []string{"system_", "registry"})
				cs, err := parser.ValidateCreateTable(tc.query, 1337)
				require.NoError(t, err)

				require.Equal(t, tc.expPrefix, cs.GetPrefix())
				require.Equal(t, tc.expStructureHash, cs.GetStructureHash())
				for _, erq := range tc.expRawQueries {
					rq, err := cs.GetRawQueryForTableID(tables.TableID(*big.NewInt(erq.id)))
					require.NoError(t, err)
					require.Equal(t, erq.rawQuery, rq)
				}
			}
		}(it))
	}
}

func TestMaxReadQuerySize(t *testing.T) {
	t.Parallel()

	maxReadQuerySize := 25
	opts := []parsing.Option{
		parsing.WithMaxReadQuerySize(maxReadQuerySize),
	}
	parser := newParser(t, []string{"system_", "registry"}, opts...)

	t.Run("success", func(t *testing.T) {
		_, err := parser.ValidateReadQuery("SELECT * FROM foo_1337_1")
		require.NoError(t, err)
	})

	t.Run("failure", func(t *testing.T) {
		_, err := parser.ValidateReadQuery("SELECT * FROM foo_1337_1 WHERE id = 1")
		var expErr *parsing.ErrReadQueryTooLong
		require.ErrorAs(t, err, &expErr)
		require.Equal(t, 37, expErr.Length)
		require.Equal(t, maxReadQuerySize, expErr.MaxAllowed)
	})
}

func TestMaxWriteQuerySize(t *testing.T) {
	t.Parallel()

	maxWriteQuerySize := 40
	opts := []parsing.Option{
		parsing.WithMaxWriteQuerySize(maxWriteQuerySize),
	}
	parser := newParser(t, []string{"system_", "registry"}, opts...)

	t.Run("success", func(t *testing.T) {
		_, err := parser.ValidateMutatingQuery("INSERT INTO foo_1337_1 VALUES ('hello')", 1337)
		require.NoError(t, err)
	})

	t.Run("failure", func(t *testing.T) {
		_, err := parser.ValidateMutatingQuery("INSERT INTO foo_1337_1 VALUES ('hello12')", 1337)
		var expErr *parsing.ErrWriteQueryTooLong
		require.ErrorAs(t, err, &expErr)
		require.Equal(t, 41, expErr.Length)
		require.Equal(t, maxWriteQuerySize, expErr.MaxAllowed)
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
			query: "update foo_1337_100 set a=1;update foo_1337_100 set b=2;",
			expectedStmts: []string{
				"update foo_1337_100 set a = 1",
				"update foo_1337_100 set b = 2",
			},
		},
		{
			name:  "insert update",
			query: "insert into foo_1337_1 values (1);update foo_1337_1 set b=2;",
			expectedStmts: []string{
				"insert into foo_1337_1 values (1)",
				"update foo_1337_1 set b = 2",
			},
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := newParser(t, []string{"system_", "registry"})
				stmts, err := parser.ValidateMutatingQuery(tc.query, 1337)
				require.NoError(t, err)

				for i := range stmts {
					query, err := stmts[i].GetQuery(nil)
					require.NoError(t, err)
					require.Equal(t, tc.expectedStmts[i], query)
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
			query:        "grant insert, UPDATE on a_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d';",
			roles:        []common.Address{common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d")},
			privileges:   []tableland.Privilege{tableland.PrivInsert, tableland.PrivUpdate},
			expectedStmt: "grant insert, update on a_1337_100 to '0xd43c59d5694ec111eb9e986c233200b14249558d'",
		},

		{
			name:  "revoke",
			query: "revoke delete on a_1337_100 from '0xd43c59d5694ec111eb9e986c233200b14249558d', '0x4afe8e30db4549384b0a05bb796468b130c7d6e0';", // nolint
			roles: []common.Address{
				common.HexToAddress("0xd43c59d5694ec111eb9e986c233200b14249558d"),
				common.HexToAddress("0x4afe8e30db4549384b0a05bb796468b130c7d6e0"),
			},
			privileges:   []tableland.Privilege{tableland.PrivDelete},
			expectedStmt: "revoke delete on a_1337_100 from '0xd43c59d5694ec111eb9e986c233200b14249558d', '0x4afe8e30db4549384b0a05bb796468b130c7d6e0'", // nolint
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()
				parser := newParser(t, []string{"system_", "registry"})
				stmts, err := parser.ValidateMutatingQuery(tc.query, 1337)
				require.NoError(t, err)

				for i := range stmts {
					gs, ok := stmts[i].(parsing.GrantStmt)
					require.True(t, ok)
					q, err := gs.GetQuery(nil)
					require.NoError(t, err)
					require.Equal(t, tc.expectedStmt, q)
					require.Equal(t, tc.roles, gs.GetRoles())
					require.ElementsMatch(t, tc.privileges, gs.GetPrivileges())
				}
			}
		}(it))
	}
}

func TestWriteStatementAddWhereClause(t *testing.T) {
	t.Parallel()

	type subTest struct {
		name        string
		query       string
		whereClause string
		expQuery    string
	}
	testCase := []subTest{
		{
			name:        "no-where-clause",
			query:       "UPDATE foo_1337_10 SET id = 1",
			whereClause: "bar = 1",
			expQuery:    "update foo_1337_10 set id = 1 where bar = 1",
		},
		{
			name:        "with-where-clause",
			query:       "update foo_1337_10 set id = 1 where bar = 1",
			whereClause: "c in (1, 2)",
			expQuery:    "update foo_1337_10 set id = 1 where bar = 1 and c in (1, 2)",
		},
	}

	for _, tc := range testCase {
		t.Run(tc.name, func(tc subTest) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				parser := newParser(t, []string{"system_", "registry"})
				mss, err := parser.ValidateMutatingQuery(tc.query, 1337)
				require.NoError(t, err)
				require.Len(t, mss, 1)

				ws, ok := mss[0].(parsing.WriteStmt)
				require.True(t, ok)

				err = ws.AddWhereClause(tc.whereClause)
				require.NoError(t, err)

				sql, err := ws.GetQuery(nil)
				require.NoError(t, err)
				require.Equal(t, tc.expQuery, sql)
			}
		}(tc))
	}
}

func TestWriteStatementAddReturningClause(t *testing.T) {
	t.Parallel()
	t.Run("insert-add-returning", func(t *testing.T) {
		t.Parallel()

		parser := newParser(t, []string{"system_", "registry"})
		mss, err := parser.ValidateMutatingQuery("insert into foo_1337_1 VALUES ('bar')", 1337)
		require.NoError(t, err)
		require.Len(t, mss, 1)

		ws, ok := mss[0].(parsing.WriteStmt)
		require.True(t, ok)

		err = ws.AddReturningClause()
		require.NoError(t, err)

		sql, err := ws.GetQuery(nil)
		require.NoError(t, err)
		require.Equal(t, "insert into foo_1337_1 values ('bar') returning (rowid)", sql)
	})

	t.Run("update-add-returning", func(t *testing.T) {
		t.Parallel()

		parser := newParser(t, []string{"system_", "registry"})
		mss, err := parser.ValidateMutatingQuery("update foo_1337_1 set foo = 'bar'", 1337)
		require.NoError(t, err)
		require.Len(t, mss, 1)

		ws, ok := mss[0].(parsing.WriteStmt)
		require.True(t, ok)

		err = ws.AddReturningClause()
		require.NoError(t, err)

		sql, err := ws.GetQuery(nil)
		require.NoError(t, err)
		require.Equal(t, "update foo_1337_1 set foo = 'bar' returning (rowid)", sql)
	})

	t.Run("delete-add-returning-error", func(t *testing.T) {
		t.Parallel()

		parser := newParser(t, []string{"system_", "registry"})
		mss, err := parser.ValidateMutatingQuery("DELETE FROM foo_1337_1 WHERE foo = 'bar'", 1337)
		require.NoError(t, err)
		require.Len(t, mss, 1)

		ws, ok := mss[0].(parsing.WriteStmt)
		require.True(t, ok)

		err = ws.AddReturningClause()
		require.ErrorAs(t, err, &parsing.ErrCantAddReturningOnDELETE)
	})
}

func TestCustomFunctionResolveReadQuery(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name     string
		query    string
		mustFail bool
		expQuery string
	}

	rqr := newReadQueryResolver(map[tableland.ChainID]int64{
		tableland.ChainID(1337): 1001,
		tableland.ChainID(1338): 1002,
		tableland.ChainID(1339): 1003,
	})
	tests := []testCase{
		{
			name:     "select with block_num(*)",
			query:    "select block_num(1337), block_num(1338) from foo_1337_1 where a = block_num(1339)",
			expQuery: "select 1001, 1002 from foo_1337_1 where a = 1003",
		},
		{
			name:     "select with block_num(*) for chainID that doesn't exist",
			query:    "select block_num(1337) from foo_1337_1 where a = block_num(1336)",
			mustFail: true,
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				parser := newParser(t, []string{"system_", "registry"})
				stmt, err := parser.ValidateReadQuery(tc.query)
				require.NoError(t, err)

				q, err := stmt.GetQuery(rqr)
				if tc.mustFail {
					require.Error(t, err)
					return
				}
				require.Equal(t, tc.expQuery, q)
			}
		}(it))
	}
}

func TestCustomFunctionResolveWriteQuery(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name       string
		query      string
		mustFail   bool
		expQueries []string
	}

	wqr := newWriteQueryResolver("0xDEADBEEF", 100)
	tests := []testCase{
		{
			name:       "insert with custom functions",
			query:      "insert into foo_1337_1 values (txn_hash(), block_num())",
			expQueries: []string{"insert into foo_1337_1 values ('0xDEADBEEF', 100)"},
		},
		{
			name:       "update with custom functions",
			query:      "update foo_1337_1 SET a=txn_hash(), b=block_num() where c in (block_num(), block_num()+1)",
			expQueries: []string{"update foo_1337_1 SET a='0xDEADBEEF', b=100 where c in (100, 100+1)"},
		},
		{
			name:       "delete with custom functions",
			query:      "delete from foo_1337_1 where a=block_num() and b=txn_hash()",
			expQueries: []string{"delete from foo_1337_1 where a=100 and b='0xDEADBEEF'"},
		},
		{
			name:  "multiple queries",
			query: "insert into foo_1337_1 values (txn_hash()); delete from foo_1337_1 where a=block_num()",
			expQueries: []string{
				"insert into foo_1337_1 values ('0xDEADBEEF')",
				"delete from foo_1337_1 where a=100",
			},
		},
		{
			name:     "block_num() with integer argument",
			query:    "delete from foo_1337_1 where a=block_num(1337)",
			mustFail: true,
		},
		{
			name:     "block_num() with string argument",
			query:    "delete from foo_1337_1 where a=block_num('foo')",
			mustFail: true,
		},
		{
			name:     "txn_hash() with an integer argument",
			query:    "insert into foo_1337_1 values (txn_hash(1))",
			mustFail: true,
		},
		{
			name:     "txn_hash() with a string argument",
			query:    "insert into foo_1337_1 values (txn_hash('foo'))",
			mustFail: true,
		},
	}

	for _, it := range tests {
		t.Run(it.name, func(tc testCase) func(t *testing.T) {
			return func(t *testing.T) {
				t.Parallel()

				parser := newParser(t, []string{"system_", "registry"})
				mutStmts, err := parser.ValidateMutatingQuery(tc.query, tableland.ChainID(100))
				require.NoError(t, err)

				for i, stmt := range mutStmts {
					q, err := stmt.GetQuery(wqr)
					if tc.mustFail {
						require.Error(t, err)
						return
					}
					require.Equal(t, tc.expQueries[i], q)
				}
			}
		}(it))
	}
}

type writeQueryResolver struct {
	txnHash     string
	blockNumber int64
}

func newWriteQueryResolver(txnHash string, blockNumber int64) *writeQueryResolver {
	return &writeQueryResolver{txnHash: txnHash, blockNumber: blockNumber}
}

func (wqr *writeQueryResolver) GetTxnHash() string {
	return wqr.txnHash
}

func (wqr *writeQueryResolver) GetBlockNumber() int64 {
	return wqr.blockNumber
}

type readQueryResolver struct {
	chainBlockNumbers map[tableland.ChainID]int64
}

func newReadQueryResolver(chainBlockNumbers map[tableland.ChainID]int64) *readQueryResolver {
	return &readQueryResolver{chainBlockNumbers: chainBlockNumbers}
}

func (wqr *readQueryResolver) GetBlockNumber(chainID tableland.ChainID) (int64, bool) {
	blockNumber, ok := wqr.chainBlockNumbers[chainID]
	return blockNumber, ok
}

func newParser(t *testing.T, prefixes []string, opts ...parsing.Option) parsing.SQLValidator {
	t.Helper()
	p, err := parser.New(prefixes, opts...)
	require.NoError(t, err)
	return p
}

// Helpers to have a pointer to pointer for generic test-case running.
func ptr2ErrInvalidSyntax() **sqlparser.ErrSyntaxError {
	var e *sqlparser.ErrSyntaxError
	return &e
}

func ptr2ErrEmptyStatement() **parsing.ErrEmptyStatement {
	var e *parsing.ErrEmptyStatement
	return &e
}

func ptr2ErrSystemTableReferencing() **parsing.ErrSystemTableReferencing {
	var e *parsing.ErrSystemTableReferencing
	return &e
}

func ptr2ErrNonDeterministicFunction() **sqlparser.ErrKeywordIsNotAllowed {
	var e *sqlparser.ErrKeywordIsNotAllowed
	return &e
}

func ptr2ErrKeywordIsNotAllowed() **sqlparser.ErrKeywordIsNotAllowed {
	var e *sqlparser.ErrKeywordIsNotAllowed
	return &e
}

func ptr2ErrSubquery() **sqlparser.ErrStatementContainsSubquery {
	var e *sqlparser.ErrStatementContainsSubquery
	return &e
}

func ptr2ErrNoTopLevelCreate() **parsing.ErrNoTopLevelCreate {
	var e *parsing.ErrNoTopLevelCreate
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

func ptr2ErrWrongFormatTableName() **sqlparser.ErrTableNameWrongFormat {
	var e *sqlparser.ErrTableNameWrongFormat
	return &e
}

func ptr2ErrPrefixTableName() **parsing.ErrPrefixTableName {
	var e *parsing.ErrPrefixTableName
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

func ptr2ErrInsertWithSelectChainMistmatch() **parsing.ErrInsertWithSelectChainMistmatch {
	var e *parsing.ErrInsertWithSelectChainMistmatch
	return &e
}
