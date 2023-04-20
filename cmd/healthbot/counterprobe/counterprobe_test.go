package counterprobe

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/client"
	clientV1 "github.com/textileio/go-tableland/pkg/client/v1"
	"github.com/textileio/go-tableland/pkg/wallet"
)

func TestProduction(t *testing.T) {
	t.SkipNow()

	ctx := context.Background()
	wallet, err := wallet.NewWallet("FILL ME")
	require.NoError(t, err)

	chain := client.Chains[client.ChainIDs.Optimism]
	client, err := clientV1.NewClient(ctx, wallet, clientV1.NewClientChain(chain))
	require.NoError(t, err)

	cp, err := New("optimism-mainnet", client, "Runbook_24", time.Second, time.Second*10, 1, 1)
	require.NoError(t, err)

	value, err := cp.healthCheck(context.Background())
	require.NoError(t, err)
	require.NotZero(t, value)
}
