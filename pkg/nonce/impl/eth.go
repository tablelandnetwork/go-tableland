package impl

import (
	"context"
	"errors"
	"fmt"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/textileio/go-tableland/pkg/nonce"
)

// EthClient is the Ethereum implementation of the Chain Client.
type EthClient struct {
	backend bind.ContractBackend
}

// NewEthClient returns am EthClient.
func NewEthClient(backend bind.ContractBackend) nonce.ChainClient {
	return &EthClient{
		backend: backend,
	}
}

// PendingNonceAt retrieves the current pending nonce associated with an account.
func (c *EthClient) PendingNonceAt(ctx context.Context, account common.Address) (int64, error) {
	nonce, err := c.backend.PendingNonceAt(ctx, account)
	if err != nil {
		return 0, fmt.Errorf("chain client pending nonce at: %s", err)
	}

	return int64(nonce), nil
}

// TransactionReceipt returns the receipt of a transaction by transaction hash.
// Note that the receipt is not available for pending transactions.
func (c *EthClient) TransactionReceipt(ctx context.Context, txHash common.Hash) (*types.Receipt, error) {
	tr, ok := c.backend.(ethereum.TransactionReader)
	if !ok {
		return nil, errors.New("chain client casting backend to transaction reader")
	}

	txReceipt, err := tr.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, fmt.Errorf("chain client get transaction receipt: %s", err)
	}

	return txReceipt, nil
}

// HeadHeader returns the latest known header of the current canonical chain.
func (c *EthClient) HeadHeader(ctx context.Context) (*types.Header, error) {
	h, err := c.backend.HeaderByNumber(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("get head header: %s", err)
	}

	return h, nil
}
