package dbhash

import (
	"bytes"
	"context"
	"crypto/sha1"
	"database/sql"
	"encoding/hex"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/tests"
)

func TestDatabaseHash(t *testing.T) {
	testCases := []testCase{
		tc0(),
		tc1(),
		tc2(),
	}

	for i, tc := range testCases {
		t.Run(fmt.Sprint(i), func(t *testing.T) {
			dbURI := tests.Sqlite3URI(t)

			db, err := sql.Open("sqlite3", dbURI)
			require.NoError(t, err)

			_, err = db.Exec(tc.dbSeed)
			require.NoError(t, err)

			tx, err := db.BeginTx(context.Background(), nil)
			defer func() {
				_ = tx.Commit()
			}()
			require.NoError(t, err)

			hash, err := DatabaseStateHash(context.Background(), tx, tc.opts...)
			require.NoError(t, err)
			require.Equal(t, tc.exp, hash)
		})
	}
}

type testCase struct {
	dbSeed string
	opts   []Option
	exp    string
}

func tc0() testCase {
	return testCase{
		dbSeed: `CREATE TABLE a (a int);
				CREATE TABLE b (c int, d text);
				INSERT INTO a VALUES (123);
				INSERT INTO a VALUES (456);
				INSERT INTO b VALUES (10, "ten");`,
		opts: []Option{
			WithFetchSchemasQuery("SELECT tbl_name, sql FROM sqlite_schema WHERE type = 'table'"),
		},
		exp: func() string {
			var buf bytes.Buffer
			buf.Write([]byte("a"))
			buf.Write([]byte("CREATE TABLE a (a int)"))
			buf.Write([]byte("123"))
			buf.Write([]byte("456"))
			buf.Write([]byte("b"))
			buf.Write([]byte("CREATE TABLE b (c int, d text)"))
			buf.Write([]byte("10"))
			buf.Write([]byte("ten"))

			h := sha1.New()
			h.Write(buf.Bytes())

			return hex.EncodeToString(h.Sum(nil))
		}(),
	}
}

// Similar to tc0. Changes seed order but get the same hash because of ORDER BY clause.
func tc1() testCase {
	return testCase{
		dbSeed: `CREATE TABLE b (c int, d text);
				CREATE TABLE a (a int);
				INSERT INTO b VALUES (10, "ten");
				INSERT INTO a VALUES (123);
				INSERT INTO a VALUES (456);
				`,
		opts: []Option{
			WithFetchSchemasQuery("SELECT tbl_name, sql FROM sqlite_schema WHERE type = 'table' ORDER BY tbl_name"),
		},
		exp: func() string {
			var buf bytes.Buffer
			buf.Write([]byte("a"))
			buf.Write([]byte("CREATE TABLE a (a int)"))
			buf.Write([]byte("123"))
			buf.Write([]byte("456"))
			buf.Write([]byte("b"))
			buf.Write([]byte("CREATE TABLE b (c int, d text)"))
			buf.Write([]byte("10"))
			buf.Write([]byte("ten"))

			h := sha1.New()
			h.Write(buf.Bytes())

			return hex.EncodeToString(h.Sum(nil))
		}(),
	}
}

// Adds a per table query function.
func tc2() testCase {
	return testCase{
		dbSeed: `CREATE TABLE a (a int);
				CREATE TABLE b (c int, d text);
				INSERT INTO a VALUES (123);
				INSERT INTO a VALUES (456);
				INSERT INTO b VALUES (10, "ten");`,
		opts: []Option{
			WithFetchSchemasQuery("SELECT tbl_name, sql FROM sqlite_schema WHERE type = 'table'"),
			WithPerTableQueryFn(func(s string) string {
				// We create a query specific for table b
				switch s {
				case "b":
					return fmt.Sprintf("SELECT d FROM %s", s) // we are only selecting the d column
				default:
					return fmt.Sprintf("SELECT * FROM %s", s)
				}
			}),
		},
		exp: func() string {
			var buf bytes.Buffer
			buf.Write([]byte("a"))
			buf.Write([]byte("CREATE TABLE a (a int)"))
			buf.Write([]byte("123"))
			buf.Write([]byte("456"))
			buf.Write([]byte("b"))
			buf.Write([]byte("CREATE TABLE b (c int, d text)"))
			// buf.Write([]byte("10")) this column value is not part of the output
			buf.Write([]byte("ten"))

			h := sha1.New()
			h.Write(buf.Bytes())

			return hex.EncodeToString(h.Sum(nil))
		}(),
	}
}
