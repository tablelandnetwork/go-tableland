package tableregistry

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

// TableRegistry defines the interface for interaction with the registry smart contract.
type TableRegistry interface {
	IsOwner(context context.Context, addrress common.Address, id *big.Int) (bool, error)
}
