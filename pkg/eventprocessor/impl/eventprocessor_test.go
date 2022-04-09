package eventprocessor

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/tableregistry/impl/testutil"
	txnpimpl "github.com/textileio/go-tableland/pkg/txn/impl"
	"github.com/textileio/go-tableland/tests"
)

// TODO(jsign): add tests
func TestBlockWithSingleEvent(t *testing.T) {
	t.Parallel()

	t.Run("success", func(t *testing.T) {

	})
	t.Run("failure", func(t *testing.T) {})
}

func TestBlockWithTwoEvents(t *testing.T) {
	t.Parallel()

	t.Run("success-success", func(t *testing.T) {})
	t.Run("failure-success", func(t *testing.T) {})
	t.Run("success-failure", func(t *testing.T) {})
}

// TODO(jsign): test exec trace

func setup(t *testing.T) *EventProcessor {
	t.Helper()

	// Spin up the EVM chain with the contract.
	backend, addr, sc, authOpts := testutil.Setup(t)

	// Spin up dependencies needed for the EventProcessor.
	// i.e: TxnProcessor, Parser, and EventFeed (connected to the EVM chain)
	ef, err := efimpl.New(backend, addr, eventfeed.WithMinBlockChainDepth(0))
	require.NoError(t, err)
	url, err := tests.PostgresURL()
	require.NoError(t, err)
	txnp, err := txnpimpl.NewTxnProcessor(url, 0)
	require.NoError(t, err)
	parser := parserimpl.New([]string{"system_", "registry"}, 0, 0)

	// Create EventProcessor for our test.
	ep, err := New(parser, txnp, ef)
	require.NoError(t, err)

	return ep
}
