package counterprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProduction(t *testing.T) {
	t.SkipNow()
	siwe := "fillme"
	endpoint := "https://testnet.tableland.network/rpc"
	tblname := "Runbook_24"

	cp, err := New("optimism-mainnet", endpoint, siwe, tblname, time.Second, time.Second*10)
	require.NoError(t, err)

	value, err := cp.healthCheck(context.Background())
	require.NoError(t, err)
	require.NotZero(t, value)
}
