package v1

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/tests/fullstack"
)

func TestCreate(t *testing.T) {
	calls := setup(t)
	requireCreate(t, calls)
}

func TestWrite(t *testing.T) {
	calls := setup(t)
	tableName := requireCreate(t, calls)
	requireWrite(t, calls, tableName)
}

func TestRead(t *testing.T) {
	t.Run("status 200", func(t *testing.T) {
		calls := setup(t)
		tableName := requireCreate(t, calls)
		hash := requireWrite(t, calls, tableName)
		requireReceipt(t, calls, hash, WaitFor(time.Second*10))

		type result struct {
			Bar string `json:"bar"`
		}

		res0 := []result{}
		calls.query(fmt.Sprintf("select * from %s", tableName), &res0)
		require.Len(t, res0, 1)
		require.Equal(t, "baz", res0[0].Bar)

		res1 := map[string]interface{}{}
		calls.query(fmt.Sprintf("select * from %s", tableName), &res1, ReadFormat(Table))
		require.Len(t, res1, 2)

		res2 := result{}
		calls.query(fmt.Sprintf("select * from %s", tableName), &res2, ReadUnwrap())
		require.Equal(t, "baz", res2.Bar)

		res3 := []string{}
		calls.query(fmt.Sprintf("select * from %s", tableName), &res3, ReadExtract())
		require.Len(t, res3, 1)
		require.Equal(t, "baz", res3[0])

		res4 := ""
		calls.query(fmt.Sprintf("select * from %s", tableName), &res4, ReadUnwrap(), ReadExtract())
		require.Equal(t, "baz", res4)
	})

	t.Run("status 400", func(t *testing.T) {
		calls := setup(t)
		err := calls.client.Read(context.Background(), "SELECTZ * FROM foo_1", struct{}{})
		require.Error(t, err)
	})
}

func TestGetReceipt(t *testing.T) {
	t.Run("status 200", func(t *testing.T) {
		calls := setup(t)
		tableName := requireCreate(t, calls)
		hash := requireWrite(t, calls, tableName)
		requireReceipt(t, calls, hash, WaitFor(time.Second*10))
	})

	t.Run("status 400", func(t *testing.T) {
		calls := setup(t)
		_ = requireCreate(t, calls)
		_, _, err := calls.client.Receipt(context.Background(), "0xINVALIDHASH")
		require.Error(t, err)
	})

	t.Run("status 404", func(t *testing.T) {
		calls := setup(t)
		_ = requireCreate(t, calls)
		_, exists, err := calls.client.Receipt(context.Background(), "0x5c6f90e52284726a7276d6a20a3df94a4532a8fa4c921233a301e95673ad0255") //nolint
		require.NoError(t, err)
		require.False(t, exists)
	})
}

func TestGetTableByID(t *testing.T) {
	t.Run("status 200", func(t *testing.T) {
		calls := setup(t)
		id, fullName := calls.create(
			"(bar text DEFAULT 'foo',zar int, CHECK (zar>0))",
			WithPrefix("foo"), WithReceiptTimeout(time.Second*10))

		table := calls.getTableByID(id)
		require.NotEmpty(t, fullName, table.Name)
		require.Equal(t, "https://testnets.tableland.network/api/v1/tables/1337/1", table.ExternalUrl)
		require.Equal(t, "https://tables.tableland.xyz/1337/1.html", table.AnimationUrl)
		require.Equal(t, "https://tables.tableland.xyz/1337/1.svg", table.Image)

		require.Len(t, table.Attributes, 1)
		require.Equal(t, "date", table.Attributes[0].DisplayType)
		require.Equal(t, "created", table.Attributes[0].TraitType)
		require.NotEmpty(t, table.Attributes[0].Value)

		require.NotNil(t, table.Schema)
		require.Len(t, table.Schema.Columns, 2)
		require.Equal(t, "bar", table.Schema.Columns[0].Name)
		require.Equal(t, "text", table.Schema.Columns[0].Type_)
		require.Len(t, table.Schema.Columns[0].Constraints, 1)
		require.Equal(t, "default 'foo'", table.Schema.Columns[0].Constraints[0])

		require.Len(t, table.Schema.TableConstraints, 1)
		require.Equal(t, "check(zar>0)", table.Schema.TableConstraints[0])
	})
	t.Run("status 404", func(t *testing.T) {
		calls := setup(t)
		id, err := NewTableID("1337")
		require.NoError(t, err)
		_, err = calls.client.GetTable(context.Background(), id)
		require.ErrorIs(t, err, ErrTableNotFound)
	})
}

func TestVersion(t *testing.T) {
	calls := setup(t)
	info, err := calls.version()
	require.NoError(t, err)

	require.Equal(t, int32(0), info.Version)
	require.NotEmpty(t, info.GitCommit)
	require.NotEmpty(t, info.GitBranch)
	require.NotEmpty(t, info.GitState)
	require.NotEmpty(t, info.GitSummary)
	require.NotEmpty(t, info.BuildDate)
	require.NotEmpty(t, info.BinaryVersion)
}

func TestHealth(t *testing.T) {
	calls := setup(t)
	healthy, err := calls.health()
	require.NoError(t, err)
	require.True(t, healthy)
}

func TestBlockNum(t *testing.T) {
	// Our initial simulated blockchain setup already produces two blocks.
	calls := setup(t)

	// We create a table and do two inserts, that will increase our block number to 5.
	tableName := requireCreate(t, calls)
	requireReceipt(t, calls, requireWrite(t, calls, tableName), WaitFor(time.Second*10))
	requireReceipt(t, calls, requireWrite(t, calls, tableName), WaitFor(time.Second*10))

	type result struct {
		BlockNumber int64 `json:"bn"`
	}

	res := []result{}
	calls.query(fmt.Sprintf("select block_num(1337) as bn from %s", tableName), &res)
	require.Len(t, res, 2)                         // it should be 2 because we inserted two rows
	require.Equal(t, int64(5), res[0].BlockNumber) // the block number should be 5
}

func requireCreate(t *testing.T, calls clientCalls) string {
	_, tableName := calls.create("(bar text)", WithPrefix("foo"), WithReceiptTimeout(time.Second*10))
	require.Equal(t, "foo_1337_1", tableName)
	return tableName
}

func requireWrite(t *testing.T, calls clientCalls, table string) string {
	hash := calls.write(fmt.Sprintf("insert into %s (bar) values('baz')", table))
	require.NotEmpty(t, hash)
	return hash
}

func requireReceipt(t *testing.T, calls clientCalls, hash string, opts ...ReceiptOption) *apiv1.TransactionReceipt {
	res, found := calls.receipt(hash, opts...)
	require.True(t, found)
	require.NotNil(t, res)
	return res
}

type clientCalls struct {
	client       *Client
	create       func(schema string, opts ...CreateOption) (TableID, string)
	write        func(query string) string
	query        func(query string, target interface{}, opts ...ReadOption)
	receipt      func(txnHash string, options ...ReceiptOption) (*apiv1.TransactionReceipt, bool)
	getTableByID func(tableID TableID) *apiv1.Table
	version      func() (*apiv1.VersionInfo, error)
	health       func() (bool, error)
}

func setup(t *testing.T) clientCalls {
	stack := fullstack.CreateFullStack(t, fullstack.Deps{})

	c := client.Chain{
		Endpoint:     stack.Server.URL,
		ID:           client.ChainID(fullstack.ChainID),
		ContractAddr: stack.Address,
	}

	client, err := NewClient(
		context.Background(),
		stack.Wallet,
		Alchemy,
		NewClientChain(c),
		NewClientContractBackend(stack.Backend))
	require.NoError(t, err)

	ctx := context.Background()
	return clientCalls{
		client: client,
		create: func(schema string, opts ...CreateOption) (TableID, string) {
			go func() {
				time.Sleep(time.Second * 1)
				stack.Backend.Commit()
			}()
			id, table, err := client.Create(ctx, schema, opts...)
			require.NoError(t, err)
			return id, table
		},
		query: func(query string, target interface{}, opts ...ReadOption) {
			err := client.Read(ctx, query, target, opts...)
			require.NoError(t, err)
		},
		write: func(query string) string {
			hash, err := client.Write(ctx, query)
			require.NoError(t, err)
			stack.Backend.Commit()
			return hash
		},
		receipt: func(txnHash string, options ...ReceiptOption) (*apiv1.TransactionReceipt, bool) {
			receipt, found, err := client.Receipt(ctx, txnHash, options...)
			require.NoError(t, err)
			return receipt, found
		},
		getTableByID: func(tableID TableID) *apiv1.Table {
			table, err := client.GetTable(ctx, tableID)
			require.NoError(t, err)
			return table
		},
		version: func() (*apiv1.VersionInfo, error) {
			return client.Version(ctx)
		},
		health: func() (bool, error) {
			return client.CheckHealth(ctx)
		},
	}
}
