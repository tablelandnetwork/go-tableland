package impl

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/klauspost/compress/zstd"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed"
	efimpl "github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl"
	"github.com/textileio/go-tableland/pkg/eventprocessor/eventfeed/impl/sqlitechainclient"
	executor "github.com/textileio/go-tableland/pkg/eventprocessor/impl/executor/impl"
	"github.com/textileio/go-tableland/pkg/parsing"
	parserimpl "github.com/textileio/go-tableland/pkg/parsing/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/tests"
)

func TestReplayProductionHistory(t *testing.T) {
	t.Parallel()

	if testing.Short() {
		t.Skipf("skipping history replay execution because running -short tests")
	}

	expectedStateHashes := map[tableland.ChainID]string{
		1:      "ce4f083e256d3458a329b6cd1ba7d8e93d9703b3",
		5:      "b42740b92e40d616061a88872e19616c05469750",
		10:     "c9afca82a8c30fc683da366b5f4920e7c131ec5d",
		69:     "643af9ad784444242c6ef415727203941a720197",
		137:    "b5fb42f3538738ab5856abf9e3b2e38d82378ca4",
		420:    "184800f533f4edd186853a85829fad8bc7802c4e",
		80001:  "df269d9451e0bf1b77c7616aff4148a3f1a4559a",
		421613: "073727932afcee9a5dba19f43c023689ca855dc2",
	}

	historyDBURI := getHistoryDBURI(t)
	// Launch the validator syncing all chains.
	eps, waitFullSync := launchValidatorForAllChainsBackedByEVMHistory(t, historyDBURI)

	// Wait for all of them to finish syncing.
	waitFullSync()

	// We compare the chain hash after full sync with the previous iteration calculated hash.
	// These should always match. If that isn't the case, it means that the chain execution is non-deterministic.
	ctx := context.Background()
	for _, ep := range eps {
		bs, err := ep.executor.NewBlockScope(ctx, ep.mLastProcessedHeight.Load()+1)
		require.NoError(t, err)

		hash, err := bs.StateHash(ctx, ep.chainID)
		require.NoError(t, err)

		assert.Equal(t, expectedStateHashes[ep.chainID], hash.Hash,
			"ChainID %d hash %s doesn't match %s", ep.chainID, hash.Hash, expectedStateHashes[ep.chainID])
		require.NoError(t, bs.Close())
	}

	// Do a graceful close, to double check closing works correctly without any blocking or delays.
	for _, ep := range eps {
		ep.Stop()
	}
}

func launchValidatorForAllChainsBackedByEVMHistory(t *testing.T, historyDBURI string) ([]*EventProcessor, func()) {
	dbURI := tests.Sqlite3URI(t)
	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxOpenConns(1)

	chains := getChains(t, historyDBURI)
	eps := make([]*EventProcessor, len(chains))
	for i, chain := range chains {
		eps[i] = spinValidatorStackForChainID(t, dbURI, historyDBURI, parser, chain.chainID, chain.scAddress, db)
	}

	waitForSynced := func() {
		var wg sync.WaitGroup
		wg.Add(len(chains))
		for i := range chains {
			go func(i int) {
				defer wg.Done()
				for {
					if eps[i].mLastProcessedHeight.Load() == chains[i].tipBlockNumber {
						return
					}
					time.Sleep(time.Second)
				}
			}(i)
		}
		wg.Wait()
	}

	return eps, waitForSynced
}

func spinValidatorStackForChainID(
	t *testing.T,
	dbURI string,
	historyDBURI string,
	parser parsing.SQLValidator,
	chainID tableland.ChainID,
	scAddress common.Address,
	db *sql.DB,
) *EventProcessor {
	ex, err := executor.NewExecutor(chainID, db, parser, 0, &aclMock{})
	require.NoError(t, err)

	systemStore, err := system.New(dbURI, chainID)
	require.NoError(t, err)

	eventBasedBackend, err := sqlitechainclient.New(historyDBURI, chainID)
	require.NoError(t, err)

	ef, err := efimpl.New(
		systemStore,
		chainID,
		eventBasedBackend,
		scAddress,
		eventfeed.WithMinBlockDepth(0))
	require.NoError(t, err)

	ep, err := New(
		parser,
		ex,
		ef,
		chainID,
		eventprocessor.WithHashCalcStep(1_000_000_000),
	)
	require.NoError(t, err)
	require.NoError(t, ep.Start())

	return ep
}

type chainIDWithTip struct {
	chainID        tableland.ChainID
	scAddress      common.Address
	tipBlockNumber int64
}

func getChains(t *testing.T, historyDBURI string) []chainIDWithTip {
	db, err := sql.Open("sqlite3", historyDBURI)
	require.NoError(t, err)

	rows, err := db.Query(`select chain_id, address, max(block_number) 
                           from system_evm_events 
                           group by chain_id, address
                           order by chain_id, block_number`)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, rows.Close())
	}()

	var chains []chainIDWithTip
	for rows.Next() {
		require.NoError(t, rows.Err())
		var chainID, blockNumber int64
		var scAddress string
		require.NoError(t, rows.Scan(&chainID, &scAddress, &blockNumber))
		chains = append(chains, chainIDWithTip{
			chainID:        tableland.ChainID(chainID),
			scAddress:      common.HexToAddress(scAddress),
			tipBlockNumber: blockNumber,
		})
	}

	return chains
}

func getHistoryDBURI(t *testing.T) string {
	zstdDB, err := os.Open("testdata/evm_history.db.zst")
	require.NoError(t, err)

	decoder, err := zstd.NewReader(zstdDB)
	require.NoError(t, err)

	// Create target database to decompress
	historyDBFilePath := filepath.Join(t.TempDir(), "evm_history.db")
	historyDB, err := os.OpenFile(historyDBFilePath, os.O_WRONLY|os.O_CREATE, 0o755)
	require.NoError(t, err)

	// Decompress
	_, err = decoder.WriteTo(historyDB)
	require.NoError(t, err)
	require.NoError(t, historyDB.Close())

	// Return full path of prepared database.
	return fmt.Sprintf("file:%s?", historyDBFilePath)
}
