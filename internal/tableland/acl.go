package tableland

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

// ACL is the API for access control rules check.
type ACL interface {
	// CheckAuthorization checks if an address is authorized to use Tableland's gateway.
	CheckAuthorization(context.Context, common.Address) error

	// IsOwner checks if an address is the owner of a table by making a contract call.
	IsOwner(context.Context, common.Address, TableID) (bool, error)

	// CheckPrivileges checks if an address can execute a specific operation on a table.
	CheckPrivileges(context.Context, common.Address, TableID, Operation) error
}

// Privilege maps to SQL privilege and is the thing needed to execute an operation.
type Privilege int

const (
	_ Privilege = iota // the zero (0) is reserved for handling special cases
	// PrivInsert represents the privilege to execute the operation OpInsert
	// The value is 1, the abbreviation is "a".
	PrivInsert

	// PrivUpdate represents the privilege to execute the operation OpUpdate
	// the value is 2, the abbreviation is "w".
	PrivUpdate

	// PrivDelete represents the privilege to execute the operation OpDelete
	// the value is 3, the abbreviation is "d".
	PrivDelete
)

// NewPrivilegeFromAbbreviation converts a privilege abbreviation into a Privilege.
func NewPrivilegeFromAbbreviation(abbreviation string) (Privilege, error) {
	switch abbreviation {
	case "a":
		return PrivInsert, nil
	case "w":
		return PrivUpdate, nil
	case "d":
		return PrivDelete, nil
	}

	return 0, fmt.Errorf("unsupported abbreviation string=%s", abbreviation)
}

// NewPrivilegeFromString converts a privilege string into a Privilege.
func NewPrivilegeFromString(s string) (Privilege, error) {
	switch s {
	case "insert":
		return PrivInsert, nil
	case "update":
		return PrivUpdate, nil
	case "delete":
		return PrivDelete, nil
	}

	return 0, fmt.Errorf("unsupported string=%s", s)
}

// String returns the string representation of a Privilege.
func (p Privilege) String() string {
	switch p {
	case PrivInsert:
		return "insert"
	case PrivUpdate:
		return "update"
	case PrivDelete:
		return "delete"
	default:
		return "nil"
	}
}

// Abbreviation returns the char that abbreviates the privilege.
func (p Privilege) Abbreviation() string {
	switch p {
	case PrivInsert:
		return "a"
	case PrivUpdate:
		return "w"
	case PrivDelete:
		return "d"
	default:
		return ""
	}
}

// Operation represents the kind of operation that can by executed in Tableland.
type Operation int

const (
	// OpSelect is represents a SELECT query.
	OpSelect Operation = iota
	// OpInsert is represents a INSERT query.
	OpInsert
	// OpUpdate is represents a UPDATE query.
	OpUpdate
	// OpDelete is represents a DELETE query.
	OpDelete
	// OpGrant is represents a GRANT query.
	OpGrant
	// OpRevoke is represents a REVOKE query.
	OpRevoke
	// OpCreate is represents a CREATE query.
	OpCreate
)

var operationPrivilegeMap map[Operation]Privilege

func init() {
	// This map gives the privilege that is needed for each operation.
	operationPrivilegeMap = map[Operation]Privilege{
		OpInsert: PrivInsert,
		OpDelete: PrivDelete,
		OpUpdate: PrivUpdate,
	}
}

// Privileges represents a list of privileges.
type Privileges []Privilege

// CanExecute checks if the list of privileges can execute a given operation.
// In case the operation cannot be executed, it returns the privilege that
// would allow the execution.
func (p Privileges) CanExecute(operation Operation) (bool, Privilege) {
	privilegeNeededForOperation := operationPrivilegeMap[operation]
	for _, privilege := range p {
		if privilege == privilegeNeededForOperation {
			return true, 0
		}
	}
	return false, privilegeNeededForOperation
}

// Abbreviations returns a slice of abbreviations.
func (p Privileges) Abbreviations() []string {
	abbreviations := make([]string, len(p))
	for i, privilege := range p {
		abbreviations[i] = privilege.Abbreviation()
	}
	return abbreviations
}
