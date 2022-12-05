package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
	"github.com/textileio/go-tableland/tests/fullstack"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	requireCreate(t, calls)
}

func TestWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table)
}

func TestRead(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	hash := requireWrite(t, calls, table)
	requireReceipt(t, calls, hash, WaitFor(time.Second*10))

	type result struct {
		Bar string `json:"bar"`
	}

	res0 := []result{}
	calls.query(fmt.Sprintf("select * from %s", table), &res0)
	require.Len(t, res0, 1)
	require.Equal(t, "baz", res0[0].Bar)

	res1 := map[string]interface{}{}
	calls.query(fmt.Sprintf("select * from %s", table), &res1, ReadOutput(Table))
	require.Len(t, res1, 2)

	res2 := result{}
	calls.query(fmt.Sprintf("select * from %s", table), &res2, ReadUnwrap())
	require.Equal(t, "baz", res2.Bar)

	res3 := []string{}
	calls.query(fmt.Sprintf("select * from %s", table), &res3, ReadExtract())
	require.Len(t, res3, 1)
	require.Equal(t, "baz", res3[0])

	res4 := ""
	calls.query(fmt.Sprintf("select * from %s", table), &res4, ReadUnwrap(), ReadExtract())
	require.Equal(t, "baz", res4)
}

func TestGetTableByID(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	id, fullName := requireCreate(t, calls)

	table := calls.getTableById(id)
	require.NotEmpty(t, fullName, table.Name)
	require.NotEmpty(t, table.ExternalUrl)
	require.NotEmpty(t, table.AnimationUrl)
	require.NotEmpty(t, table.Image)
	require.Greater(t, len(table.Attributes), 0)

	require.NotNil(t, table.Schema)
	require.NotEmpty(t, table.Schema.Columns)
	require.NotEmpty(t, table.Schema.TableConstraints)
}

func TestVersion(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	info, err := calls.version()
	require.NoError(t, err)

	require.NotEmpty(t, info.Version)
	require.NotEmpty(t, info.GitCommit)
	require.NotEmpty(t, info.GitBranch)
	require.NotEmpty(t, info.GitState)
	require.NotEmpty(t, info.GitSummary)
	require.NotEmpty(t, info.BuildDate)
	require.NotEmpty(t, info.BinaryVersion)
}

func TestHealth(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	healthy, err := calls.health()
	require.NoError(t, err)
	require.True(t, healthy)
}

func requireCreate(t *testing.T, calls clientCalls) (TableID, string) {
	id, tableName := calls.create(
		"(bar text DEFAULT 'foo',zar int, CHECK (zar>0))",
		WithPrefix("foo"), WithReceiptTimeout(time.Second*10))
	require.Equal(t, "foo_1337_1", tableName)
	return id, tableName
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
	create       func(schema string, opts ...CreateOption) (TableID, string)
	write        func(query string) string
	query        func(query string, target interface{}, opts ...ReadOption)
	receipt      func(txnHash string, options ...ReceiptOption) (*apiv1.TransactionReceipt, bool)
	getTableById func(tableID TableID) *apiv1.Table
	version      func() (*apiv1.VersionInfo, error)
	health       func() (bool, error)
}

func setup(t *testing.T) clientCalls {
	stack := fullstack.CreateFullStack(t, fullstack.Deps{})

	c := Chain{
		Endpoint:     stack.Server.URL,
		ID:           ChainID(fullstack.ChainID),
		ContractAddr: stack.Address,
	}

	client, err := NewClient(context.Background(), stack.Wallet, NewClientChain(c), NewClientContractBackend(stack.Backend))
	require.NoError(t, err)

	ctx := context.Background()
	return clientCalls{
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
		getTableById: func(tableID TableID) *apiv1.Table {
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
