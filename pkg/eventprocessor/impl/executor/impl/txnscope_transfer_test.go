package impl

import (
	"context"
	"fmt"
	"math/big"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
)

func TestChangeTableOwner(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	tableID := tableland.TableID(*big.NewInt(100))
	txnp, _, db := newExecutorWithTable(t, 0)

	require.Equal(t, 1,
		tableRowCountT100(
			t,
			db,
			fmt.Sprintf(
				"select count(1) from registry WHERE controller = '%s' and id = %s and chain_id = %d",
				"0xb451cee4A42A652Fe77d373BAe66D42fd6B8D8FF",
				tableID.String(),
				chainID,
			),
		))

	b, err := txnp.OpenBatch(ctx)
	require.NoError(t, err)

	// change table's owner
	err = b.ChangeTableOwner(ctx, tableID, controller)
	require.NoError(t, err)
	require.NoError(t, b.Commit())
	require.NoError(t, b.Close())
	require.NoError(t, txnp.Close(ctx))

	require.Equal(t, 1,
		tableRowCountT100(
			t,
			db,
			fmt.Sprintf(
				"select count(1) from registry WHERE controller = '%s' and id = %s and chain_id = %d",
				controller.Hex(),
				tableID.String(),
				chainID,
			),
		))
}
