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
	store    sqlstore.SQLStore
	registry tableregistry.TableRegistry
}

// NewACL creates a new instance of the ACL.
func NewACL(store sqlstore.SQLStore, registry tableregistry.TableRegistry) tableland.ACL {
	return &acl{
		store,
		registry,
	}
}

// CheckAuthorization checks if an address is authorized to use Tableland's gateway.
func (acl *acl) CheckAuthorization(ctx context.Context, controller common.Address) error {
	res, err := acl.store.IsAuthorized(ctx, controller.String())
	if err != nil {
		return fmt.Errorf("failed to check if address is authorized: %s", err)
	}

	if !res.IsAuthorized {
		return fmt.Errorf("address not authorized")
	}

	return nil
}

// IsOwner checks if an address is the owner of a table by making a contract call.
func (acl *acl) IsOwner(ctx context.Context, controller common.Address, id tableland.TableID) (bool, error) {
	isOwner, err := acl.registry.IsOwner(ctx, controller, id.ToBigInt())
	if err != nil {
		return false, fmt.Errorf("failed to execute contract call: %s", err)
	}
	return isOwner, nil
}

// CheckPrivileges checks if an address can execute a specific operation on a table.
func (acl *acl) CheckPrivileges(
	ctx context.Context,
	tx pgx.Tx,
	controller common.Address,
	id tableland.TableID,
	op tableland.Operation) error {
	aclRule, err := acl.store.WithTx(tx).GetACLOnTableByController(ctx, id, controller.String())
	if err != nil {
		return fmt.Errorf("privileges lookup: %s", err)
	}

	isAllowed, missingPrivilege := aclRule.Privileges.CanExecute(op)
	if !isAllowed {
		return fmt.Errorf("cannot execute operation, missing privilege=%s", missingPrivilege.ToSQLString())
	}

	return nil
}
