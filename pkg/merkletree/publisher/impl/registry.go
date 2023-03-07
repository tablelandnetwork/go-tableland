package impl

import (
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	"github.com/rs/zerolog"
	logger "github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/wallet"
)

// MerkleRootRegistryLogger is an implementat that simply logs the roots.
type MerkleRootRegistryLogger struct {
	logger zerolog.Logger
}

// NewMerkleRootRegistryLogger creates a new MerkleRootRegistryLogger.
func NewMerkleRootRegistryLogger(logger zerolog.Logger) *MerkleRootRegistryLogger {
	return &MerkleRootRegistryLogger{logger: logger}
}

// Publish logs the roots.
func (r *MerkleRootRegistryLogger) Publish(
	_ context.Context,
	chainID int64,
	blockNumber int64,
	tables []*big.Int,
	roots [][]byte,
) error {
	tableIds := make([]int64, len(tables))
	for i, id := range tables {
		tableIds[i] = id.Int64()
	}

	l := r.logger.Info().
		Int64("chain_id", chainID).
		Int64("block_number", blockNumber).
		Ints64("tables", tableIds)

	for i, root := range roots {
		l.Hex(fmt.Sprintf("root_%d", tables[i].Int64()), root)
	}

	l.Msg("merkle roots")

	return nil
}

// MerkleRootRegistryEthereum is a Ethereum Root Registry implementation.
type MerkleRootRegistryEthereum struct {
	contract *Contract
	backend  bind.ContractBackend
	wallet   *wallet.Wallet
	tracker  nonce.NonceTracker

	log zerolog.Logger
}

// NewMerkleRootRegistryEthereum creates a new MerkleRootRegistryEthereum.
func NewMerkleRootRegistryEthereum(
	backend bind.ContractBackend,
	contractAddr common.Address,
	wallet *wallet.Wallet,
	tracker nonce.NonceTracker,
) (*MerkleRootRegistryEthereum, error) {
	contract, err := NewContract(contractAddr, backend)
	if err != nil {
		return nil, fmt.Errorf("creating contract: %v", err)
	}

	log := logger.With().
		Str("component", "merklerootregistryethereum").
		Logger()

	return &MerkleRootRegistryEthereum{
		contract: contract,
		backend:  backend,
		wallet:   wallet,
		tracker:  tracker,
		log:      log,
	}, nil
}

// Publish publishes the roots to a Smart Contract.
func (r *MerkleRootRegistryEthereum) Publish(
	ctx context.Context,
	chainID int64,
	_ int64,
	tables []*big.Int,
	roots [][]byte,
) error {
	transactOpts, err := bind.NewKeyedTransactorWithChainID(r.wallet.PrivateKey(), big.NewInt(chainID))
	if err != nil {
		return fmt.Errorf("creating keyed transactor: %s", err)
	}

	gasTipCap, err := r.backend.SuggestGasTipCap(ctx)
	if err != nil {
		return fmt.Errorf("suggest gas price: %s", err)
	}

	_, err = r.callWithRetry(ctx, func() (*types.Transaction, error) {
		registerPendingTx, unlock, nonce := r.tracker.GetNonce(ctx)
		defer unlock()

		opts := &bind.TransactOpts{
			Context:   ctx,
			Signer:    transactOpts.Signer,
			From:      transactOpts.From,
			Nonce:     big.NewInt(0).SetInt64(nonce),
			GasTipCap: gasTipCap,
		}

		rootsCopy := make([][32]byte, len(roots))
		for i, root := range roots {
			copy(rootsCopy[i][:], root[:])
		}

		tx, err := r.contract.SetRoots(opts, tables, rootsCopy)
		if err != nil {
			return nil, err
		}
		registerPendingTx(tx.Hash())
		return tx, nil
	})
	if err != nil {
		return fmt.Errorf("retryable SetRoots call: %s", err)
	}
	return nil
}

func (r *MerkleRootRegistryEthereum) callWithRetry(
	ctx context.Context, f func() (*types.Transaction, error),
) (*types.Transaction, error) {
	tx, err := f()

	possibleErrMgs := []string{"nonce too low", "invalid transaction nonce"}
	if err != nil {
		for _, errMsg := range possibleErrMgs {
			if strings.Contains(err.Error(), errMsg) {
				r.log.Warn().Err(err).Msg("retrying smart contract call")
				if err := r.tracker.Resync(ctx); err != nil {
					return nil, fmt.Errorf("resync: %s", err)
				}
				tx, err = f()
				if err != nil {
					return nil, fmt.Errorf("retry contract call: %s", err)
				}

				return tx, nil
			}
		}

		return nil, fmt.Errorf("contract call: %s", err)
	}

	return tx, nil
}
