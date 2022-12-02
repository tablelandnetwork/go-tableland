package client

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
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
	t.Fail()
}

func TestVersion(t *testing.T) {
	t.Fail()
}

func TestHealth(t *testing.T) {
	t.Fail()
}

func requireCreate(t *testing.T, calls clientCalls) (TableID, string) {
	id, table := calls.create("(bar text)", WithPrefix("foo"), WithReceiptTimeout(time.Second*10))
	require.Equal(t, "foo_1337_1", table)
	return id, table
}

func requireWrite(t *testing.T, calls clientCalls, table string) string {
	hash := calls.write(fmt.Sprintf("insert into %s (bar) values('baz')", table))
	require.NotEmpty(t, hash)
	return hash
}

func requireReceipt(t *testing.T, calls clientCalls, hash string, opts ...ReceiptOption) *TxnReceipt {
	res, found := calls.receipt(hash, opts...)
	require.True(t, found)
	require.NotNil(t, res)
	return res
}

type clientCalls struct {
	create       func(schema string, opts ...CreateOption) (TableID, string)
	write        func(query string) string
	query        func(query string, target interface{}, opts ...ReadOption)
	receipt      func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool)
	getTableById func(tableID int64) (*TxnReceipt, bool)
	version      func() error
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
	t.Cleanup(func() {
		client.Close()
	})

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
		receipt: func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool) {
			receipt, found, err := client.Receipt(ctx, txnHash, options...)
			require.NoError(t, err)
			return receipt, found
		},
		getTableById: func(tableID int64) (*TxnReceipt, bool) { panic("TODO") },
		version:      func() error { panic("TODO") },
		health:       func() (bool, error) { panic("TODO") },
	}
}
