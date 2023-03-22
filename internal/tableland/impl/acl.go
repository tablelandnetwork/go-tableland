package impl

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sqlstore"
	"github.com/textileio/go-tableland/pkg/tables"
)

type acl struct {
	store sqlstore.SystemStore
}

// NewACL creates a new instance of the ACL.
func NewACL(store sqlstore.SystemStore) tableland.ACL {
	return &acl{
		store: store,
	}
}

// CheckPrivileges checks if an address can execute a specific operation on a table.
func (acl *acl) CheckPrivileges(
	ctx context.Context,
	tx *sql.Tx,
	controller common.Address,
	id tables.TableID,
	op tableland.Operation,
) (bool, error) {
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
