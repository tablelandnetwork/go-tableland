package impl

import (
	"context"
	"encoding/hex"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/nonce"
	sqlstoreimpl "github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/ethereum"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/testutil"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func TestTracker(t *testing.T) {
	ctx := context.Background()
	tracker, backend, contract, txOpts, wallet := setup(ctx, t)

	fn1, unlock1, nonce1 := tracker.GetNonce(ctx)
	txn1, err := contract.RunSQL(txOpts, "tbl", wallet.Address(), "INSERT ...")
	require.NoError(t, err)
	backend.Commit()
	fn1(txn1.Hash())
	unlock1()

	fn2, unlock2, nonce2 := tracker.GetNonce(ctx)
	txn2, err := contract.RunSQL(txOpts, "tbl", wallet.Address(), "INSERT ...")
	require.NoError(t, err)
	backend.Commit()
	fn2(txn2.Hash())
	unlock2()

	fn3, unlock3, nonce3 := tracker.GetNonce(ctx)
	txn3, err := contract.RunSQL(txOpts, "tbl", wallet.Address(), "INSERT ...")
	require.NoError(t, err)
	backend.Commit()
	fn3(txn3.Hash())
	unlock3()

	require.Equal(t, int64(0), nonce1)
	require.Equal(t, int64(1), nonce2)
	require.Equal(t, int64(2), nonce3)
	require.Eventually(t, func() bool {
		return tracker.GetPendingCount(ctx) == 0
	}, 5*time.Second, time.Second)
}

func TestTrackerUnlock(t *testing.T) {
	ctx := context.Background()
	tracker, backend, contract, txOpts, wallet := setup(ctx, t)

	_, unlock, nonce1 := tracker.GetNonce(ctx)
	// this go routine simulates a concurrent runSQL call that went wrong
	go func() {
		time.Sleep(1 * time.Second)
		unlock()
	}()

	// this call will be blocked until nonce tracker is unblocked
	fn2, unlock2, nonce2 := tracker.GetNonce(ctx)
	txn2, err := contract.RunSQL(txOpts, "tbl", wallet.Address(), "INSERT ...")
	require.NoError(t, err)
	backend.Commit()
	fn2(txn2.Hash())
	unlock2()

	require.Equal(t, int64(0), nonce1)
	require.Equal(t, int64(0), nonce2) // nonce2 should not have been incremented
	require.Eventually(t, func() bool {
		return tracker.GetPendingCount(ctx) == 0
	}, 5*time.Second, time.Second)
}

func TestTrackerPendingTxGotStuck(t *testing.T) {
	ctx := context.Background()
	tracker, backend, contract, txOpts, wallet := setup(ctx, t)

	fn1, unlock1, nonce1 := tracker.GetNonce(ctx)
	txn1, err := contract.RunSQL(txOpts, "tbl", wallet.Address(), "INSERT ...")
	require.NoError(t, err)
	backend.Commit()
	fn1(txn1.Hash())
	unlock1()

	fn2, unlock2, nonce2 := tracker.GetNonce(ctx)
	txn2, err := contract.RunSQL(txOpts, "tbl", wallet.Address(), "INSERT ...")
	require.NoError(t, err)
	//backend.Commit() , this tx will get stuck
	fn2(txn2.Hash())
	unlock2()

	require.Equal(t, int64(0), nonce1)
	require.Equal(t, int64(1), nonce2)
	require.Eventually(t, func() bool {
		return tracker.GetPendingCount(ctx) == 1
	}, 5*time.Second, time.Second)
}

func setup(ctx context.Context, t *testing.T) (
	nonce.NonceTracker,
	*backends.SimulatedBackend,
	*ethereum.Contract,
	*bind.TransactOpts,
	*wallet.Wallet) {
	url, err := tests.PostgresURL()
	require.NoError(t, err)

	backend, _, contract, txOpts, _ := testutil.Setup(t)

	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	wallet, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(key)))
	require.NoError(t, err)

	sqlstore, err := sqlstoreimpl.New(ctx, url)
	require.NoError(t, err)

	tracker, err := NewLocalTracker(
		ctx,
		wallet,
		&NonceStore{sqlstore},
		backend,
		500*time.Millisecond,
		0,
		24*time.Hour)
	require.NoError(t, err)

	return tracker, backend, contract, txOpts, wallet
}
