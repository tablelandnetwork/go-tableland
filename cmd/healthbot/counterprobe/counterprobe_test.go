package counterprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func TestProduction(t *testing.T) {
	t.SkipNow()
	pk := "fillme"
	wallet, err := wallet.NewWallet(pk)
	require.NoError(t, err)
	tblname := "Runbook_24"

	cp, err := New(context.Background(), "optimism-mainnet", wallet, tblname, time.Second, time.Second*10)
	require.NoError(t, err)

	value, err := cp.healthCheck(context.Background())
	require.NoError(t, err)
	require.NotZero(t, value)
}
