package impl

import (
	"bytes"
	"encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/pkg/merkletree/publisher"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl"
	"github.com/textileio/go-tableland/tests"
)

func TestPublisher(t *testing.T) {
	// We are going to pass this logger to MerkleRootRegistryLogger.
	// It will fill the `buf` with logged bytes, that later we can inspect that it logged the expected values.
	var buf buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	helper := setup(t, []publisher.TreeLeaves{
		{
			TablePrefix: "",
			ChainID:     1,
			TableID:     1,
			BlockNumber: 1,
			Leaves:      []byte("ABCDEFGHABCDEFGH"),
		},
		{
			TablePrefix: "",
			ChainID:     1,
			TableID:     2,
			BlockNumber: 1,
			Leaves:      []byte("ABCDEFGHABCDEFGH"),
		},
	})

	p := publisher.NewMerkleRootPublisher(helper.store, NewMerkleRootRegistryLogger(logger), time.Second)
	p.Start()
	defer p.Close()

	type l struct {
		ChainID     int    `json:"chain_id"`
		BlockNumber int    `json:"block_number"`
		Level       string `json:"level"`
		Message     string `json:"message"`
		Root1       string `json:"root_1"`
		Root2       string `json:"root_2"`
		Tables      []int  `json:"tables"`
	}

	// Eventually the MerkleRootLogger will build the tree and emit the expected log.
	require.Eventually(t, func() bool {
		// We're going to inspect `buf`.
		if buf.Len() != 0 {
			expLog := &l{}
			decoder := json.NewDecoder(bytes.NewReader(buf.Bytes()))
			require.NoError(t, decoder.Decode(expLog))

			require.Equal(t, 1, expLog.ChainID)
			require.Equal(t, 1, expLog.BlockNumber)
			require.Equal(t, "info", expLog.Level)
			require.Equal(t, "merkle roots", expLog.Message)
			require.Equal(t, "8b8e53316fb13d0bfe0e559e947f729af5296981a47095be51054afae8e48ab1", expLog.Root1)
			require.Equal(t, "8b8e53316fb13d0bfe0e559e947f729af5296981a47095be51054afae8e48ab1", expLog.Root2)
			require.Equal(t, []int{1, 2}, expLog.Tables)

			helper.assertTreeLeavesIsEmpty(t)
		}
		return buf.Len() != 0
	}, 10*time.Second, time.Second)
}

func setup(t *testing.T, data []publisher.TreeLeaves) *helper {
	t.Helper()

	sqlite, err := impl.NewSQLiteDB(tests.Sqlite3URI(t))
	require.NoError(t, err)

	// pre populate system_tree_leaves with provided data
	for _, treeLeaves := range data {
		_, err = sqlite.DB.Exec(
			"INSERT INTO system_tree_leaves (prefix, chain_id, table_id, block_number, leaves) VALUES (?1, ?2, ?3, ?4, ?5)",
			treeLeaves.TablePrefix,
			treeLeaves.ChainID,
			treeLeaves.TableID,
			treeLeaves.BlockNumber,
			treeLeaves.Leaves,
		)
		require.NoError(t, err)
	}

	return &helper{
		db:    sqlite,
		store: NewLeavesStore(sqlite),
	}
}

type helper struct {
	db    *impl.SQLiteDB
	store *LeavesStore
}

func (h *helper) assertTreeLeavesIsEmpty(t *testing.T) {
	var count int
	err := h.db.DB.QueryRow("SELECT count(1) FROM system_tree_leaves").Scan(&count)
	require.NoError(t, err)
	require.Equal(t, 0, count)
}

// We need a thread-safe version of bytes.Buffer to avoid data races in this test.
// The reason for that is because there's a thread writing to the buffer and another one reading from it.
type buffer struct {
	b bytes.Buffer
	m sync.Mutex
}

func (b *buffer) Read(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Read(p)
}

func (b *buffer) Write(p []byte) (n int, err error) {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Write(p)
}

func (b *buffer) Bytes() []byte {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Bytes()
}

func (b *buffer) Len() int {
	b.m.Lock()
	defer b.m.Unlock()
	return b.b.Len()
}
