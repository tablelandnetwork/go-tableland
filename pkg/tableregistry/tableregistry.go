package tableregistry

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type TableRegistry interface {
	IsOwner(context context.Context, addrress common.Address, id *big.Int) (bool, error)
}
