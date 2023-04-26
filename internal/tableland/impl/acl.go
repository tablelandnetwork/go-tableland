package impl

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/database"
	"github.com/textileio/go-tableland/pkg/database/db"
	"github.com/textileio/go-tableland/pkg/tables"
)

// ACLStore has access to the stored acl information.
type ACLStore struct {
	db *database.SQLiteDB
}

// NewACL creates a new instance of the ACL.
func NewACL(db *database.SQLiteDB) *ACLStore {
	return &ACLStore{
		db: db,
	}
}

var _ tableland.ACL = (*ACLStore)(nil)

// CheckPrivileges checks if an address can execute a specific operation on a table.
func (acl *ACLStore) CheckPrivileges(
	ctx context.Context,
	tx *sql.Tx,
	chainID tableland.ChainID,
	controller common.Address,
	id tables.TableID,
	op tableland.Operation,
) (bool, error) {
	row, err := acl.db.Queries.WithTx(tx).GetAclByTableAndController(ctx, db.GetAclByTableAndControllerParams{
		ChainID: int64(chainID),
		TableID: id.ToBigInt().Int64(),
		UPPER:   controller.Hex(),
	})
	if err == sql.ErrNoRows {
		return false, nil
	}

	if err != nil {
		return false, fmt.Errorf("privileges lookup: %s", err)
	}

	aclRule, err := transformToObject(row)
	if err != nil {
		return false, fmt.Errorf("transforming to dto: %s", err)
	}

	isAllowed, _ := aclRule.Privileges.CanExecute(op)
	if !isAllowed {
		return false, nil
	}

	return true, nil
}

// transforms the ACL data transfer object to ACL object model.
func transformToObject(acl db.SystemAcl) (SystemACL, error) {
	id, err := tables.NewTableIDFromInt64(acl.TableID)
	if err != nil {
		return SystemACL{}, fmt.Errorf("parsing id to string: %s", err)
	}

	var privileges tableland.Privileges
	if acl.Privileges&tableland.PrivInsert.Bitfield > 0 {
		privileges = append(privileges, tableland.PrivInsert)
	}
	if acl.Privileges&tableland.PrivUpdate.Bitfield > 0 {
		privileges = append(privileges, tableland.PrivUpdate)
	}
	if acl.Privileges&tableland.PrivDelete.Bitfield > 0 {
		privileges = append(privileges, tableland.PrivDelete)
	}

	systemACL := SystemACL{
		ChainID:    tableland.ChainID(acl.ChainID),
		TableID:    id,
		Controller: acl.Controller,
		Privileges: privileges,
	}

	return systemACL, nil
}

// SystemACL represents the system acl table.
type SystemACL struct {
	Controller string
	ChainID    tableland.ChainID
	TableID    tables.TableID
	Privileges tableland.Privileges
}
