package client_legacy

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/client"
	"github.com/textileio/go-tableland/tests/fullstack"
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
	requireWrite(t, calls, table, WriteRelay(true))
}

func TestDirectWrite(t *testing.T) {
	t.Parallel()

	calls := setup(t)
	_, table := requireCreate(t, calls)
	requireWrite(t, calls, table, WriteRelay(false))
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
	calls.read(fmt.Sprintf("select * from %s", table), &res0)
	require.Len(t, res0, 1)
	require.Equal(t, "baz", res0[0].Bar)

	res1 := map[string]interface{}{}
	calls.read(fmt.Sprintf("select * from %s", table), &res1, ReadOutput(Table))
	require.Len(t, res1, 2)

	res2 := result{}
	calls.read(fmt.Sprintf("select * from %s", table), &res2, ReadUnwrap())
	require.Equal(t, "baz", res2.Bar)

	res3 := []string{}
	calls.read(fmt.Sprintf("select * from %s", table), &res3, ReadExtract())
	require.Len(t, res3, 1)
	require.Equal(t, "baz", res3[0])

	res4 := ""
	calls.read(fmt.Sprintf("select * from %s", table), &res4, ReadUnwrap(), ReadExtract())
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

func requireCreate(t *testing.T, calls clientCalls) (TableID, string) {
	id, table := calls.create("(bar text)", WithPrefix("foo"), WithReceiptTimeout(time.Second*10))
	require.Equal(t, "foo_1337_1", table)
	return id, table
}

func requireWrite(t *testing.T, calls clientCalls, table string, opts ...WriteOption) string {
	hash := calls.write(fmt.Sprintf("insert into %s (bar) values('baz')", table), opts...)
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
	list          func() []TableInfo
	create        func(schema string, opts ...CreateOption) (TableID, string)
	read          func(query string, target interface{}, opts ...ReadOption)
	write         func(query string, opts ...WriteOption) string
	hash          func(statement string) string
	validate      func(statement string) TableID
	receipt       func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool)
	setController func(controller common.Address, tableID TableID) string
}

func setup(t *testing.T) clientCalls {
	stack := fullstack.CreateFullStack(t, fullstack.Deps{})

	c := client.Chain{
		Endpoint:     stack.Server.URL,
		ID:           client.ChainID(fullstack.ChainID),
		ContractAddr: stack.Address,
	}

	client, err := NewClient(context.Background(), stack.Wallet, NewClientChain(c), NewClientContractBackend(stack.Backend))
	require.NoError(t, err)
	t.Cleanup(func() {
		client.Close()
	})

	ctx := context.Background()
	return clientCalls{
		list: func() []TableInfo {
			res, err := client.List(ctx)
			require.NoError(t, err)
			return res
		},
		create: func(schema string, opts ...CreateOption) (TableID, string) {
			go func() {
				time.Sleep(time.Second * 1)
				stack.Backend.Commit()
			}()
			id, table, err := client.Create(ctx, schema, opts...)
			require.NoError(t, err)
			return id, table
		},
		read: func(query string, target interface{}, opts ...ReadOption) {
			err := client.Read(ctx, query, target, opts...)
			require.NoError(t, err)
		},
		write: func(query string, opts ...WriteOption) string {
			hash, err := client.Write(ctx, query, opts...)
			require.NoError(t, err)
			stack.Backend.Commit()
			return hash
		},
		hash: func(statement string) string {
			hash, err := client.Hash(ctx, statement)
			require.NoError(t, err)
			return hash
		},
		validate: func(statement string) TableID {
			tableID, err := client.Validate(ctx, statement)
			require.NoError(t, err)
			return tableID
		},
		receipt: func(txnHash string, options ...ReceiptOption) (*TxnReceipt, bool) {
			receipt, found, err := client.Receipt(ctx, txnHash, options...)
			require.NoError(t, err)
			return receipt, found
		},
		setController: func(controller common.Address, tableID TableID) string {
			hash, err := client.SetController(ctx, controller, tableID)
			require.NoError(t, err)
			stack.Backend.Commit()
			return hash
		},
	}
}
