package ethereum

import (
	"bytes"
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/nonce"
	"github.com/textileio/go-tableland/pkg/tableregistry"
	"github.com/textileio/go-tableland/pkg/wallet"
)

// Client is the Ethereum implementation of the registry client.
type Client struct {
	contract *Contract
	backend  bind.ContractBackend
	wallet   *wallet.Wallet
	chainID  int64
	tracker  nonce.NonceTracker
}

// NewClient creates a new Client.
func NewClient(
	backend bind.ContractBackend,
	chainID int64,
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

	auth, err := bind.NewKeyedTransactorWithChainID(c.wallet.PrivateKey(), big.NewInt(c.chainID))
	if err != nil {
		return nil, fmt.Errorf("creating keyed transactor: %s", err)
	}

	registerPendingTx, unlock, nonce := c.tracker.GetNonce(ctx)
	defer unlock()

	opts := &bind.TransactOpts{
		Context:  ctx,
		Signer:   auth.Signer,
		From:     auth.From,
		Nonce:    big.NewInt(0).SetInt64(nonce),
		GasPrice: gasPrice,
	}

	tx, err := c.contract.RunSQL(opts, table.String(), addr, statement)
	if err != nil {
		return nil, fmt.Errorf("calling RunSQL: %v", err)
	}
	registerPendingTx(tx.Hash())

	return tx, nil
}
