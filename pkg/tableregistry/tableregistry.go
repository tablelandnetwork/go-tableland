package tableregistry

import (
	"context"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
)

// Transaction represents a Smart Contract transaction.
type Transaction interface {
	Hash() common.Hash
}

// TableRegistry defines the interface for interaction with the registry smart contract.
type TableRegistry interface {
	IsOwner(context context.Context, addrress common.Address, id *big.Int) (bool, error)

	// RunSQL sends a transaction with a SQL statement to the Tabeland Smart Contract.
	RunSQL(context.Context, common.Address, tableland.TableID, string) (Transaction, error)
}
