package impl

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/jackc/pgx/v4"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tableregistry"
)

type acl struct {
	chainID  tableland.ChainID
	store    sqlstore.SQLStore
	registry tableregistry.TableRegistry
}

// NewACL creates a new instance of the ACL.
func NewACL(chainID tableland.ChainID, store sqlstore.SQLStore, registry tableregistry.TableRegistry) tableland.ACL {
	return &acl{
		chainID:  chainID,
		store:    store,
		registry: registry,
	}
}

// CheckPrivileges checks if an address can execute a specific operation on a table.
func (acl *acl) CheckPrivileges(
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) (bool, error) {
	aclRule, err := acl.store.WithTx(tx).GetACLOnTableByController(ctx, id, controller.String())
	if err != nil {
		return false, fmt.Errorf("privileges lookup: %s", err)
	}

	isAllowed, _ := aclRule.Privileges.CanExecute(op)
	if !isAllowed {
		return false, nil
	}

	return true, nil
}
