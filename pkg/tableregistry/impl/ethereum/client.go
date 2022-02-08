package ethereum

import (
	"bytes"
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
	owner, err := c.contract.OwnerOf(opts, id)
	if err != nil {
		return false, fmt.Errorf("calling OwnderOf: %v", err)
	}
	return bytes.Equal(addr.Bytes(), owner.Bytes()), nil
}
