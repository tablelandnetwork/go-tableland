package ethereum

import (
	"bytes"
	"context"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/rs/zerolog/log"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/tableregistry"
	"github.com/textileio/go-tableland/pkg/wallet"
)

type EthClient interface {
	bind.ContractBackend
	TransactionByHash(ctx context.Context, txHash common.Hash) (*types.Transaction, bool, error)
}

// Client is the Ethereum implementation of the registry client.
type Client struct {
	contract *Contract
	backend  EthClient
	wallet   *wallet.Wallet
	chainID  tableland.ChainID
	tracker  nonce.NonceTracker
}

// NewClient creates a new Client.
func NewClient(
	backend EthClient,
	chainID tableland.ChainID,
	contractAddr common.Address,
	wallet *wallet.Wallet,
	tracker nonce.NonceTracker) (*Client, error) {
	contract, err := NewContract(contractAddr, backend)
	if err != nil {
		return nil, fmt.Errorf("creating contract: %v", err)
	}
	return &Client{
		contract: contract,
		backend:  backend,
		wallet:   wallet,
		chainID:  chainID,
		tracker:  tracker,
	}, nil
}

// IsOwner implements IsOwner.
func (c *Client) IsOwner(context context.Context, addr common.Address, id *big.Int) (bool, error) {
	opts := &bind.CallOpts{Context: context}
	owner, err := c.contract.OwnerOf(opts, id)
	if err != nil {
		return false, fmt.Errorf("calling OwnderOf: %v", err)
	}
	return bytes.Equal(addr.Bytes(), owner.Bytes()), nil
}

// RunSQL sends a transaction with a SQL statement to the Tabeland Smart Contract.
func (c *Client) RunSQL(
	ctx context.Context,
	addr common.Address,
	table tableland.TableID,
	statement string) (tableregistry.Transaction, error) {
	gasPrice, err := c.backend.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %s", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(c.wallet.PrivateKey(), big.NewInt(int64(c.chainID)))
	if err != nil {
		return nil, fmt.Errorf("creating keyed transactor: %s", err)
	}

	tx, err := c.callWithRetry(ctx, func() (*types.Transaction, error) {
		registerPendingTx, unlock, nonce := c.tracker.GetNonce(ctx)
		defer unlock()

		opts := &bind.TransactOpts{
			Context:  ctx,
			Signer:   auth.Signer,
			From:     auth.From,
			Nonce:    big.NewInt(0).SetInt64(nonce),
			GasPrice: gasPrice,
		}

		tx, err := c.contract.RunSQL(opts, addr, table.ToBigInt(), statement)
		if err != nil {
			return nil, err
		}
		registerPendingTx(tx.Hash())
		return tx, nil
	})

	if err != nil {
		return nil, fmt.Errorf("retryable RunSQL call: %s", err)
	}
	return tx, nil
}

// SetController sends a transaction that sets the controller for a token id in Smart Contract.
func (c *Client) SetController(
	ctx context.Context,
	caller common.Address,
	table tableland.TableID,
	controller common.Address) (tableregistry.Transaction, error) {
	gasPrice, err := c.backend.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("suggest gas price: %s", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(c.wallet.PrivateKey(), big.NewInt(int64(c.chainID)))
	if err != nil {
		return nil, fmt.Errorf("creating keyed transactor: %s", err)
	}

	tx, err := c.callWithRetry(ctx, func() (*types.Transaction, error) {
		registerPendingTx, unlock, nonce := c.tracker.GetNonce(ctx)
		defer unlock()

		opts := &bind.TransactOpts{
			Context:  ctx,
			Signer:   auth.Signer,
			From:     auth.From,
			Nonce:    big.NewInt(0).SetInt64(nonce),
			GasPrice: gasPrice,
		}

		tx, err := c.contract.SetController(opts, caller, table.ToBigInt(), controller)
		if err != nil {
			return nil, err
		}
		registerPendingTx(tx.Hash())

		return tx, nil
	})
	if err != nil {
		return nil, fmt.Errorf("retryable SetController call: %v", err)
	}

	return tx, nil
}

func (c *Client) callWithRetry(ctx context.Context, f func() (*types.Transaction, error)) (*types.Transaction, error) {
	tx, err := f()

	possibleErrMgs := []string{"nonce too low", "invalid transaction nonce"}
	if err != nil {
		for _, errMsg := range possibleErrMgs {
			if strings.Contains(err.Error(), errMsg) {
				log.Warn().Err(err).Msg("retrying smart contract call")
				if err := c.tracker.Resync(ctx); err != nil {
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

func (c *Client) BumpTxnGas(ctx context.Context, txnHash common.Hash) (common.Hash, error) {
	pendingTxn, isPending, err := c.backend.TransactionByHash(ctx, txnHash)
	if err != nil {
		return common.Hash{}, fmt.Errorf("get pending txn from the mempool: %s", err)
	}
	if !isPending {
		return common.Hash{}, fmt.Errorf("the transaction hash %s isn't pending", txnHash)
	}

	fmt.Printf("pendingTxn: %v\n", pendingTxn.GasPrice())

	ltxn := &types.LegacyTx{
		Nonce:    pendingTxn.Nonce(),
		GasPrice: big.NewInt(0).Div(big.NewInt(0).Mul(pendingTxn.GasPrice(), big.NewInt(125)), big.NewInt(100)),
		Gas:      pendingTxn.Gas(),
		To:       pendingTxn.To(),
		Value:    pendingTxn.Value(),
		Data:     pendingTxn.Data(),
	}

	signer := types.NewLondonSigner(big.NewInt(int64(c.chainID)))
	txn, err := types.SignTx(types.NewTx(ltxn), signer, c.wallet.PrivateKey())
	if err != nil {
		return common.Hash{}, fmt.Errorf("signing txn: %s", err)
	}

	if err := c.backend.SendTransaction(ctx, txn); err != nil {
		return common.Hash{}, fmt.Errorf("sending txn: %s", err)
	}

	return txn.Hash(), nil
}
