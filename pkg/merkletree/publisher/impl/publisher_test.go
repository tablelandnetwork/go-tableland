package impl

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"math/big"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/accounts/abi/bind/backends"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/rs/zerolog"
	"github.com/stretchr/testify/require"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/merkletree/publisher"
	nonceimpl "github.com/textileio/go-tableland/pkg/nonce/impl"
	"github.com/textileio/go-tableland/pkg/sqlstore/impl/system"
	"github.com/textileio/go-tableland/pkg/wallet"
	"github.com/textileio/go-tableland/tests"
)

func TestPublisher(t *testing.T) {
	t.Parallel()
	// We are going to pass this logger to MerkleRootRegistryLogger.
	// It will fill the `buf` with logged bytes, that later we can inspect that it logged the expected values.
	var buf buffer
	logger := zerolog.New(&buf).With().Timestamp().Logger()

	helper := setup(t, []publisher.TreeLeaves{
		{
			TablePrefix: "",
			ChainID:     1,
			TableID:     big.NewInt(1),
			BlockNumber: 1,
			Leaves:      []byte("ABCDEFGHABCDEFGH"),
		},
		{
			TablePrefix: "",
			ChainID:     1,
			TableID:     big.NewInt(2),
			BlockNumber: 1,
			Leaves:      []byte("ABCDEFGHABCDEFGH"),
		},
	})

	p := publisher.NewMerkleRootPublisher(
		helper.leavesStore, helper.treeStore, NewMerkleRootRegistryLogger(logger), time.Second,
	)
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

			return helper.treeLeavesCount(t) == 0
		}
		return buf.Len() != 0
	}, 10*time.Second, time.Second)
}

func TestPublisherWithSimulatedBackend(t *testing.T) {
	t.Parallel()

	helper := setup(t, []publisher.TreeLeaves{
		{
			TablePrefix: "",
			ChainID:     1337,
			TableID:     big.NewInt(1),
			BlockNumber: 1,
			Leaves:      []byte("ABCDEFGHABCDEFGH"),
		},
		{
			TablePrefix: "",
			ChainID:     1337,
			TableID:     big.NewInt(2),
			BlockNumber: 1,
			Leaves:      []byte("ABCDEFGHABCDEFGH"),
		},
	})

	p := publisher.NewMerkleRootPublisher(helper.leavesStore, helper.treeStore, helper.rootRegistry, time.Second)
	p.Start()
	defer p.Close()

	// Eventually the MerkleRootLogger will build the tree and emit the expected log.
	require.Eventually(t, func() bool {
		return helper.treeLeavesCount(t) == 0
	}, 10*time.Second, time.Second)
}

func setup(t *testing.T, data []publisher.TreeLeaves) *helper {
	t.Helper()

	chain := tests.NewSimulatedChain(t)
	contract, err := chain.DeployContract(t,
		func(auth *bind.TransactOpts, sb *backends.SimulatedBackend) (common.Address, interface{}, error) {
			addr, _, contract, err := DeployContract(auth, sb)
			return addr, contract, err
		})
	require.NoError(t, err)

	privateKey := chain.CreateAccountWithBalance(t)

	w, err := wallet.NewWallet(hex.EncodeToString(crypto.FromECDSA(privateKey)))
	require.NoError(t, err)

	url := tests.Sqlite3URI(t)

	systemStore, err := system.New(url, tableland.ChainID(1337))
	require.NoError(t, err)

	tracker, err := nonceimpl.NewLocalTracker(
		context.Background(),
		w,
		nonceimpl.NewNonceStore(systemStore),
		tableland.ChainID(1337),
		chain.Backend,
		5*time.Second,
		0,
		3*time.Microsecond,
	)
	require.NoError(t, err)

	rootRegistry, err := NewMerkleRootRegistryEthereum(chain.Backend, contract.ContractAddr, w, tracker)
	require.NoError(t, err)

	db, err := sql.Open("sqlite3", url)
	require.NoError(t, err)

	// pre populate system_tree_leaves with provided data
	for _, treeLeaves := range data {
		_, err = db.Exec(
			"INSERT INTO system_tree_leaves (prefix, chain_id, table_id, block_number, leaves) VALUES (?1, ?2, ?3, ?4, ?5)",
			treeLeaves.TablePrefix,
			treeLeaves.ChainID,
			treeLeaves.TableID.Int64(),
			treeLeaves.BlockNumber,
			treeLeaves.Leaves,
		)
		require.NoError(t, err)
	}

	treeStore, err := NewMerkleTreeStore(tempfile(t))
	require.NoError(t, err)

	return &helper{
		db:           db,
		leavesStore:  NewLeavesStore(systemStore),
		treeStore:    treeStore,
		rootRegistry: rootRegistry,
	}
}

type helper struct {
	db           *sql.DB
	leavesStore  *LeavesStore
	treeStore    *MerkleTreeStore
	rootRegistry *MerkleRootRegistryEthereum
}

func (h *helper) treeLeavesCount(t *testing.T) int {
	var count int
	err := h.db.QueryRow("SELECT count(1) FROM system_tree_leaves").Scan(&count)
	require.NoError(t, err)
	return count
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

// tempfile returns a temporary file path.
func tempfile(t *testing.T) string {
	t.Helper()

	f, err := os.CreateTemp(t.TempDir(), "bolt_*.db")
	require.NoError(t, err)
	require.NoError(t, f.Close())

	return f.Name()
}
