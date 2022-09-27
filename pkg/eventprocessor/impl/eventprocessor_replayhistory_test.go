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
	if testing.Short() {
		t.Skipf("skipping history replay execution because running -short tests")
	}

	expectedStateHashes := map[tableland.ChainID]string{
		1:      "87b02f2755e043a7d7f544bb9bf79765115f9b58",
		5:      "b6f5f703af0e92d8f28773a024ed45119cef8c61",
		10:     "57555e08de5b37270ddad6c2cad2c1ae7ca6901e",
		69:     "f17f7c999790277cc91351a798eea2d08abe4285",
		137:    "241425c72e622bb8574f7fe15ccf251ddc5c6367",
		420:    "923d03c96ca0c3fa848b651697cf925bcdee5eff",
		80001:  "7b15ca5f14bc4e7475d48fface3ac1a7022e3a27",
		421613: "df1fe80afc8d9fc0ae31057b82766dd82d81ad63",
	}

	historyDBURI := getHistoryDBURI(t)
	for i := 0; i < 1; i++ {
		// Launch the validator syncing all chains.
		eps, waitFullSync := launchValidatorForAllChainsBackedByEVMHistory(t, historyDBURI)

		// Wait for all of them to finish syncing.
		waitFullSync()

		// We compare the chain hash after full sync with the previous iteration calcualted hash.
		// These should always match. If that isn't the case, it means that the chain execution is non-deterministic.
		ctx := context.Background()
		for _, ep := range eps {
			bs, err := ep.executor.NewBlockScope(ctx, ep.mLastProcessedHeight.Load()+1)
			require.NoError(t, err)

			hash, err := ep.calculateHash(ctx, bs)
			require.NoError(t, err)

			require.Equal(t, expectedStateHashes[ep.chainID], hash,
				"ChainID %d hash %s doesn't match %s", ep.chainID, hash, expectedStateHashes[ep.chainID])
			require.NoError(t, bs.Close())
		}

		// Do a graceful close, to double check closing works correctly without any blocking or delays.
		for _, ep := range eps {
			ep.Stop()
		}
	}
}

func launchValidatorForAllChainsBackedByEVMHistory(t *testing.T, historyDBURI string) ([]*EventProcessor, func()) {
	dbURI := tests.Sqlite3URI()
	parser, err := parserimpl.New([]string{"system_", "registry", "sqlite_"})
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", dbURI)
	require.NoError(t, err)
	db.SetMaxIdleConns(0)
	db.SetMaxOpenConns(1)

	chains := getChains(t, historyDBURI)
	eps := make([]*EventProcessor, len(chains))
	for i, chain := range chains {
		eps[i] = spinValidatorStackForChainID(t, dbURI, historyDBURI, parser, chain.chainID, db)
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
	db *sql.DB,
) *EventProcessor {
	ex, err := executor.NewExecutor(chainID, db, parser, 0, &aclMock{})
	require.NoError(t, err)

	systemStore, err := system.New(dbURI, tableland.ChainID(chainID))
	require.NoError(t, err)

	eventBasedBackend, err := sqlitechainclient.New(historyDBURI, chainID)
	require.NoError(t, err)

	ef, err := efimpl.New(
		systemStore,
		chainID,
		eventBasedBackend,
		common.HexToAddress("ignored"),
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
	tipBlockNumber int64
}

func getChains(t *testing.T, historyDBURI string) []chainIDWithTip {
	db, err := sql.Open("sqlite3", historyDBURI)
	require.NoError(t, err)

	rows, err := db.Query("select chain_id, max(block_number) from system_evm_events group by chain_id")
	require.NoError(t, err)
	defer rows.Close()

	var chains []chainIDWithTip
	for rows.Next() {
		require.NoError(t, rows.Err())
		var chainID, blockNumber int64
		require.NoError(t, rows.Scan(&chainID, &blockNumber))
		chains = append(chains, chainIDWithTip{
			chainID:        tableland.ChainID(chainID),
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
