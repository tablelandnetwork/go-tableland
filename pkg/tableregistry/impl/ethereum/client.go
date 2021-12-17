package ethereum

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
)

// Client is the Ethereum implementation of the registry client.
type Client struct {
	contract *Contract
}

// NewClient creates a new Client.
func NewClient(backend bind.ContractBackend, contractAddr common.Address) (*Client, error) {
	contract, err := NewContract(contractAddr, backend)
	if err != nil {
		return nil, fmt.Errorf("creating contract: %v", err)
	}
	return &Client{contract: contract}, nil
}

// IsOwner implements IsOwner.
func (c *Client) IsOwner(context context.Context, addr common.Address, id *big.Int) (bool, error) {
	opts := &bind.CallOpts{Context: context}
	bal, err := c.contract.BalanceOf(opts, addr, id)
	if err != nil {
		return false, fmt.Errorf("calling BalanceOf: %v", err)
	}
	return bal.Cmp(big.NewInt(0)) > 0, nil
}
