package client_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/pkg/testutils"
)

func TestCreate(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	requireCreate(t, calls)
}

func TestList(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	requireCreate(t, calls)
	res := calls.list()
	require.Len(t, res, 1)
}

func TestRelayWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table, client.WriteRelay(true))
}

func TestDirectWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table, client.WriteRelay(false))
}

func TestRead(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	hash := requireWrite(t, calls, table)
	requireReceipt(t, calls, hash, client.WaitFor(time.Second*10))

	type result struct {
		Bar string `json:"bar"`
	}

	res0 := []result{}
	calls.read(fmt.Sprintf("select * from %s", table), &res0)
	require.Len(t, res0, 1)
	require.Equal(t, "baz", res0[0].Bar)

	res1 := map[string]interface{}{}
	calls.read(fmt.Sprintf("select * from %s", table), &res1, client.ReadOutput(client.Table))
	require.Len(t, res1, 2)

	res2 := result{}
	calls.read(fmt.Sprintf("select * from %s", table), &res2, client.ReadUnwrap())
	require.Equal(t, "baz", res2.Bar)

	res3 := []string{}
	calls.read(fmt.Sprintf("select * from %s", table), &res3, client.ReadExtract())
	require.Len(t, res3, 1)
	require.Equal(t, "baz", res3[0])

	res4 := ""
	calls.read(fmt.Sprintf("select * from %s", table), &res4, client.ReadUnwrap(), client.ReadExtract())
	require.Equal(t, "baz", res4)
}

func TestHash(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	hash := calls.hash("create table foo_1337 (bar text)")
	require.NotEmpty(t, hash)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	id, table := requireCreate(t, calls)
	res := calls.validate(fmt.Sprintf("insert into %s (bar) values ('hi')", table))
	require.Equal(t, id, res)
}

func TestSetController(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	tableID, _ := requireCreate(t, calls)
	key, err := crypto.GenerateKey()
	require.NoError(t, err)
	controller := common.HexToAddress(crypto.PubkeyToAddress(key.PublicKey).Hex())
	hash := calls.setController(controller, tableID)
	require.NotEmpty(t, hash)
}

func requireCreate(t *testing.T, calls clientCalls) (client.TableID, string) {
	id, table := calls.create("(bar text)", client.WithPrefix("foo"), client.WithReceiptTimeout(time.Second*10))
	require.Equal(t, "foo_1337_1", table)
	return id, table
}

func requireWrite(t *testing.T, calls clientCalls, table string, opts ...client.WriteOption) string {
	hash := calls.write(fmt.Sprintf("insert into %s (bar) values('baz')", table), opts...)
	require.NotEmpty(t, hash)
	return hash
}

func requireReceipt(t *testing.T, calls clientCalls, hash string, opts ...client.ReceiptOption) *client.TxnReceipt {
	res, found := calls.receipt(hash, opts...)
	require.True(t, found)
	require.NotNil(t, res)
	return res
}

type clientCalls struct {
	list          func() []client.TableInfo
	create        func(schema string, opts ...client.CreateOption) (client.TableID, string)
	read          func(query string, target interface{}, opts ...client.ReadOption)
	write         func(query string, opts ...client.WriteOption) string
	hash          func(statement string) string
	validate      func(statement string) client.TableID
	receipt       func(txnHash string, options ...client.ReceiptOption) (*client.TxnReceipt, bool)
	setController func(controller common.Address, tableID client.TableID) string
}

func setup(t *testing.T) clientCalls {
	t.Helper()

	ctx := context.Background()

	stack := testutils.CreateFullStack(t, testutils.Deps{})

	return clientCalls{
		list: func() []client.TableInfo {
			res, err := stack.Client.List(ctx)
			require.NoError(t, err)
			return res
		},
		create: func(schema string, opts ...client.CreateOption) (client.TableID, string) {
			go func() {
				time.Sleep(time.Second * 1)
				stack.Backend.Commit()
			}()
			id, table, err := stack.Client.Create(ctx, schema, opts...)
			require.NoError(t, err)
			return id, table
		},
		read: func(query string, target interface{}, opts ...client.ReadOption) {
			err := stack.Client.Read(ctx, query, target, opts...)
			require.NoError(t, err)
		},
		write: func(query string, opts ...client.WriteOption) string {
			hash, err := stack.Client.Write(ctx, query, opts...)
			require.NoError(t, err)
			stack.Backend.Commit()
			return hash
		},
		hash: func(statement string) string {
			hash, err := stack.Client.Hash(ctx, statement)
			require.NoError(t, err)
			return hash
		},
		validate: func(statement string) client.TableID {
			tableID, err := stack.Client.Validate(ctx, statement)
			require.NoError(t, err)
			return tableID
		},
		receipt: func(txnHash string, options ...client.ReceiptOption) (*client.TxnReceipt, bool) {
			receipt, found, err := stack.Client.Receipt(ctx, txnHash, options...)
			require.NoError(t, err)
			return receipt, found
		},
		setController: func(controller common.Address, tableID client.TableID) string {
			hash, err := stack.Client.SetController(ctx, controller, tableID)
			require.NoError(t, err)
			stack.Backend.Commit()
			return hash
		},
	}
}
