package counterprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestProduction(t *testing.T) {
	t.SkipNow()
	jwt := "fillme"
	endpoint := "https://testnet.tableland.network/rpc"
	tblname := "Runbook_24"

	cp, err := New(time.Second, endpoint, jwt, tblname)
	require.NoError(t, err)

	err = cp.healthCheck(context.Background())
	require.NoError(t, err)
}
