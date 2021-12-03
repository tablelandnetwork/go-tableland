package erc1155

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
	bal, err := c.contract.BalanceOf(opts, addr, id)
	if err != nil {
		return false, fmt.Errorf("calling BalanceOf: %v", err)
	}
	return bal.Cmp(big.NewInt(0)) > 0, nil
}
