package erc721

import (
	"context"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

type Client struct {
	contract *Contract
}

func NewClient(ethEndpoint string, contractAddr common.Address) (*Client, error) {
	conn, err := ethclient.Dial(ethEndpoint)
	if err != nil {
		return nil, fmt.Errorf("dialing eth endpoint: %v", err)
	}
	contract, err := NewContract(contractAddr, conn)
	if err != nil {
		return nil, fmt.Errorf("creating contract: %v", err)
	}
	return &Client{contract: contract}, nil
}

func (c *Client) IsOwner(context context.Context, addr common.Address, id *big.Int) (bool, error) {
	opts := &bind.CallOpts{Context: context}
	owner, err := c.contract.OwnerOf(opts, id)
	if err != nil {
		return false, fmt.Errorf("calling OwnerOf: %v", err)
	}
	return owner == addr, nil
}
