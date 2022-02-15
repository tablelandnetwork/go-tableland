package node

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/pkg/parsing"
)

func CheckNonEmptyStatement(parsed *pg_query.ParseResult) error {
	if len(parsed.Stmts) == 0 {
		return &parsing.ErrEmptyStatement{}
	}
	return nil
}

func CheckSingleStatement(parsed *pg_query.ParseResult) error {
	if len(parsed.Stmts) != 1 {
		return &parsing.ErrNoSingleStatement{}
	}
	return nil
}

func CheckTopLevelUpdateInsertDelete(node *pg_query.Node) error {
	if node.GetUpdateStmt() == nil &&
		node.GetInsertStmt() == nil &&
		node.GetDeleteStmt() == nil {
		return &parsing.ErrNoTopLevelUpdateInsertDelete{}
	}
	return nil
}

func CheckTopLevelCreate(node *pg_query.Node) error {
	if node.GetCreateStmt() == nil {
		return &parsing.ErrNoTopLevelCreate{}
	}
	return nil
}

func CheckNoForUpdateOrShare(node *pg_query.SelectStmt) error {
	if node == nil {
		return ErrEmptyNode
	}

	if len(node.LockingClause) > 0 {
		return &parsing.ErrNoForUpdateOrShare{}
	}
	return nil
}

func CheckNoReturningClause(node *pg_query.Node) error {
	if node == nil {
		return ErrEmptyNode
	}

	if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if len(updateStmt.ReturningList) > 0 {
			return &parsing.ErrReturningClause{}
		}
	} else if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if len(insertStmt.ReturningList) > 0 {
			return &parsing.ErrReturningClause{}
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if len(deleteStmt.ReturningList) > 0 {
			return &parsing.ErrReturningClause{}
		}
	} else {
		return ErrUnexpectedNodeType
	}
	return nil
}

func CheckNoSystemTablesReferencing(node *pg_query.Node, systemTablePrefix string) error {
	if node == nil {
		return nil
	}
	if rangeVar := node.GetRangeVar(); rangeVar != nil {
		if strings.HasPrefix(rangeVar.Relname, systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
	} else if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if strings.HasPrefix(insertStmt.Relation.Relname, systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		return CheckNoSystemTablesReferencing(insertStmt.SelectStmt, systemTablePrefix)
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		for _, fcn := range selectStmt.FromClause {
			if err := CheckNoSystemTablesReferencing(fcn, systemTablePrefix); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if strings.HasPrefix(updateStmt.Relation.Relname, systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := CheckNoSystemTablesReferencing(fcn, systemTablePrefix); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if strings.HasPrefix(deleteStmt.Relation.Relname, systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		if err := CheckNoSystemTablesReferencing(deleteStmt.WhereClause, systemTablePrefix); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := CheckNoSystemTablesReferencing(rangeSubselectStmt.Subquery, systemTablePrefix); err != nil {
			return fmt.Errorf("subquery: %w", err)
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := CheckNoSystemTablesReferencing(joinExpr.Larg, systemTablePrefix); err != nil {
			return fmt.Errorf("join left arg: %w", err)
		}
		if err := CheckNoSystemTablesReferencing(joinExpr.Rarg, systemTablePrefix); err != nil {
			return fmt.Errorf("join right arg: %w", err)
		}
	}
	return nil
}

func getReferencedTable(node *pg_query.Node) (string, error) {
	if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		return insertStmt.Relation.Relname, nil
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		return updateStmt.Relation.Relname, nil
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		return deleteStmt.Relation.Relname, nil
	}
	return "", fmt.Errorf("the statement isn't an insert/update/delete")
}

func CheckMaxTextValueLength(node *pg_query.Node, maxLength int) error {
	if maxLength == 0 {
		return nil
	}
	if insertStmt := node.GetInsertStmt(); insertStmt != nil {
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
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
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

func checkAConstStringLength(n *pg_query.Node, maxLength int) error {
	if aConst := n.GetAConst(); aConst != nil {
		if str := aConst.Val.GetString_(); str != nil {
			if len(str.Str) > maxLength {
				return &parsing.ErrTextTooLong{
					Length:     len(str.Str),
					MaxAllowed: maxLength,
				}
			}
		}
	}
	return nil
}

// CheckNonDeterministicFunctions walks the query tree and disallow references to
// functions that aren't deterministic.
func CheckNonDeterministicFunctions(node *pg_query.Node) error {
	if node == nil {
		return nil
	}
	if sqlValFunc := node.GetSqlvalueFunction(); sqlValFunc != nil {
		return &parsing.ErrNonDeterministicFunction{}
	} else if listStmt := node.GetList(); listStmt != nil {
		for _, item := range listStmt.Items {
			if err := CheckNonDeterministicFunctions(item); err != nil {
				return fmt.Errorf("list item: %w", err)
			}
		}
	}
	if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		return CheckNonDeterministicFunctions(insertStmt.SelectStmt)
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		for _, nl := range selectStmt.ValuesLists {
			if err := CheckNonDeterministicFunctions(nl); err != nil {
				return fmt.Errorf("value list: %w", err)
			}
		}
		for _, fcn := range selectStmt.FromClause {
			if err := CheckNonDeterministicFunctions(fcn); err != nil {
				return fmt.Errorf("from: %w", err)
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		for _, t := range updateStmt.TargetList {
			if err := CheckNonDeterministicFunctions(t); err != nil {
				return fmt.Errorf("target: %w", err)
			}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := CheckNonDeterministicFunctions(fcn); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
		if err := CheckNonDeterministicFunctions(updateStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if err := CheckNonDeterministicFunctions(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := CheckNonDeterministicFunctions(rangeSubselectStmt.Subquery); err != nil {
			return fmt.Errorf("subquery: %w", err)
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := CheckNonDeterministicFunctions(joinExpr.Larg); err != nil {
			return fmt.Errorf("join left tree: %w", err)
		}
		if err := CheckNonDeterministicFunctions(joinExpr.Rarg); err != nil {
			return fmt.Errorf("join right tree: %w", err)
		}
	} else if aExpr := node.GetAExpr(); aExpr != nil {
		if err := CheckNonDeterministicFunctions(aExpr.Lexpr); err != nil {
			return fmt.Errorf("aexpr left: %w", err)
		}
		if err := CheckNonDeterministicFunctions(aExpr.Rexpr); err != nil {
			return fmt.Errorf("aexpr right: %w", err)
		}
	} else if resTarget := node.GetResTarget(); resTarget != nil {
		if err := CheckNonDeterministicFunctions(resTarget.Val); err != nil {
			return fmt.Errorf("target: %w", err)
		}
	}
	return nil
}

func CheckNoJoinOrSubquery(node *pg_query.Node) error {
	if node == nil {
		return nil
	}

	if resTarget := node.GetResTarget(); resTarget != nil {
		if err := CheckNoJoinOrSubquery(resTarget.Val); err != nil {
			return fmt.Errorf("column sub-query: %w", err)
		}
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		if len(selectStmt.ValuesLists) == 0 {
			return &parsing.ErrJoinOrSubquery{}
		}
	} else if subSelectStmt := node.GetRangeSubselect(); subSelectStmt != nil {
		return &parsing.ErrJoinOrSubquery{}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		return &parsing.ErrJoinOrSubquery{}
	} else if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if err := CheckNoJoinOrSubquery(insertStmt.SelectStmt); err != nil {
			return fmt.Errorf("insert select expr: %w", err)
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if len(updateStmt.FromClause) != 0 {
			return &parsing.ErrJoinOrSubquery{}
		}
		if err := CheckNoJoinOrSubquery(updateStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if err := CheckNoJoinOrSubquery(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if aExpr := node.GetAExpr(); aExpr != nil {
		if err := CheckNoJoinOrSubquery(aExpr.Lexpr); err != nil {
			return fmt.Errorf("aexpr left: %w", err)
		}
		if err := CheckNoJoinOrSubquery(aExpr.Rexpr); err != nil {
			return fmt.Errorf("aexpr right: %w", err)
		}
	} else if subLinkExpr := node.GetSubLink(); subLinkExpr != nil {
		return &parsing.ErrJoinOrSubquery{}
	} else if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.Args {
			if err := CheckNoJoinOrSubquery(arg); err != nil {
				return fmt.Errorf("bool expr: %w", err)
			}
		}
	}
	return nil
}
