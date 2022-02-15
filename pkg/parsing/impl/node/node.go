package node

import (
	"errors"
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/pkg/parsing"
)

var (
	ErrEmptyNode          = errors.New("empty node")
	ErrUnexpectedNodeType = errors.New("unexpected node type")
)

type Option interface{}

// TopLevelRunSQLNode represents a valid SQL statement for RunSQL calls.
type TopLevelRunSQLNode interface {
	// IsRead returns true for SELECT statements
	IsRead() bool

	// IsWrite returns true for INSERT, UPDATE and DELETE statements
	IsWrite() bool

	// IsGrant returns true for GRANT or REVOKE statements
	IsGrant() bool

	// CheckRules checks if the statement is allowed according to Tableland's rules
	CheckRules(...Option) error

	// GetReferencedTable gets the table refereced in the statement
	GetReferencedTable() (string, error)
}

func NewTopLevelRunSQLNode(n *pg_query.Node) (TopLevelRunSQLNode, error) {
	if n == nil {
		return nil, ErrEmptyNode
	}

	// if n.GetSelectStmt() != nil {
	// 	return &readNode{n}, nil
	// }

	if n.GetUpdateStmt() != nil ||
		n.GetInsertStmt() != nil ||
		n.GetDeleteStmt() != nil {
		return &writeNode{n}, nil
	}

	if n.GetGrantStmt() != nil {
		return &grantNode{n.GetGrantStmt()}, nil
	}

	return nil, &parsing.ErrNoTopLevelUpdateInsertDelete{}
}

type grantNode struct {
	*pg_query.GrantStmt
}

func (n *grantNode) CheckRules(options ...Option) error {
	privilegesStmts := n.GrantStmt.GetPrivileges()
	if len(privilegesStmts) == 0 {
		return errors.New("no privileges")
	}

	for _, privilegeStmt := range privilegesStmts {
		if privilegeStmt.GetAccessPriv().GetPrivName() != "select" {
			return errors.New("only select is allowed")
		}
	}

	return nil
}

func (n *grantNode) IsRead() bool {
	return false
}

func (n *grantNode) IsWrite() bool {
	return false
}

func (n *grantNode) IsGrant() bool {
	return true
}

func (n *grantNode) GetReferencedTable() (string, error) {
	return n.GrantStmt.GetObjects()[0].GetRangeVar().GetRelname(), nil
}

type writeNode struct {
	*pg_query.Node
}

func (n *writeNode) CheckRules(options ...Option) error {
	if err := CheckNoJoinOrSubquery(n.Node); err != nil {
		return fmt.Errorf("join or subquery check: %w", err)
	}

	if err := n.checkNoReturningClause(); err != nil {
		return fmt.Errorf("no returning clause check: %w", err)
	}

	if err := CheckNoSystemTablesReferencing(n.Node, options[0].(string)); err != nil {
		return fmt.Errorf("no system-table reference: %w", err)
	}

	if err := CheckNonDeterministicFunctions(n.Node); err != nil {
		return fmt.Errorf("no non-deterministic func check: %w", err)
	}

	if err := n.CheckMaxTextValueLength(options[1].(int)); err != nil {
		return fmt.Errorf("max text length check: %w", err)
	}

	return nil
}

func (n *writeNode) CheckMaxTextValueLength(maxLength int) error {
	if maxLength == 0 {
		return nil
	}
	if insertStmt := n.Node.GetInsertStmt(); insertStmt != nil {
		if selStmt := insertStmt.SelectStmt.GetSelectStmt(); selStmt != nil {
			for _, vl := range selStmt.ValuesLists {
				if list := vl.GetList(); list != nil {
					for _, item := range list.Items {
						if err := checkAConstStringLength(item, maxLength); err != nil {
							return err
						}
					}
				}
			}
		}
	} else if updateStmt := n.Node.GetUpdateStmt(); updateStmt != nil {
		for _, target := range updateStmt.TargetList {
			if resTarget := target.GetResTarget(); resTarget != nil {
				if err := checkAConstStringLength(resTarget.Val, maxLength); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (n *writeNode) checkNoReturningClause() error {
	if updateStmt := n.Node.GetUpdateStmt(); updateStmt != nil {
		if len(updateStmt.ReturningList) > 0 {
			return &parsing.ErrReturningClause{}
		}
	} else if insertStmt := n.Node.GetInsertStmt(); insertStmt != nil {
		if len(insertStmt.ReturningList) > 0 {
			return &parsing.ErrReturningClause{}
		}
	} else if deleteStmt := n.Node.GetDeleteStmt(); deleteStmt != nil {
		if len(deleteStmt.ReturningList) > 0 {
			return &parsing.ErrReturningClause{}
		}
	} else {
		return ErrUnexpectedNodeType
	}
	return nil
}

func (n *writeNode) IsRead() bool {
	return false
}

func (n *writeNode) IsWrite() bool {
	return true
}

func (n *writeNode) IsGrant() bool {
	return false
}

func (n *writeNode) GetReferencedTable() (string, error) {
	if insertStmt := n.GetInsertStmt(); insertStmt != nil {
		return insertStmt.Relation.Relname, nil
	} else if updateStmt := n.GetUpdateStmt(); updateStmt != nil {
		return updateStmt.Relation.Relname, nil
	} else if deleteStmt := n.GetDeleteStmt(); deleteStmt != nil {
		return deleteStmt.Relation.Relname, nil
	}
	return "", fmt.Errorf("the statement isn't an insert/update/delete")
}
