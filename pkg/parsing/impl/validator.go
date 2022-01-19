package impl

import (
	"errors"
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/pkg/parsing"
)

var (
	errEmptyNode          = errors.New("empty node")
	errUnexpectedNodeType = errors.New("unexpected node type")
)

// QueryValidator enforces PostgresSQL constraints for Tableland.
type QueryValidator struct {
	systemTablePrefix  string
	acceptedTypesNames []string
}

var _ parsing.SQLValidator = (*QueryValidator)(nil)

// New returns a Tableland query validator.
func New(systemTablePrefix string) *QueryValidator {
	// We create here a flattened slice of all the accepted type names from
	// the parsing.AcceptedTypes source of truth. We do this since having a
	// slice is easier and faster to do checks.
	var acceptedTypesNames []string
	for _, at := range parsing.AcceptedTypes {
		acceptedTypesNames = append(acceptedTypesNames, at.Names...)
	}

	return &QueryValidator{
		systemTablePrefix:  systemTablePrefix,
		acceptedTypesNames: acceptedTypesNames,
	}
}

// ValidateCreateTable validates the provided query and returns an error
// if the CREATE statement isn't allowed. Returns nil otherwise.
func (pp *QueryValidator) ValidateCreateTable(query string) error {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := pp.checkSingleStatement(parsed); err != nil {
		return fmt.Errorf("single-statement check: %w", err)
	}

	stmt := parsed.Stmts[0].Stmt
	if err := pp.checkTopLevelCreate(stmt); err != nil {
		return fmt.Errorf("allowed top level stmt: %w", err)
	}

	if err := pp.checkCreateColTypes(stmt.GetCreateStmt()); err != nil {
		return fmt.Errorf("disallowed column types: %w", err)
	}

	return nil
}

// ValidateRunSQL validates the query and returns an error if isn't allowed.
// If the query validates correctly, it returns the query type and nil.
func (pp *QueryValidator) ValidateRunSQL(query string) (parsing.QueryType, error) {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return parsing.UndefinedQuery, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := pp.checkSingleStatement(parsed); err != nil {
		return parsing.UndefinedQuery, fmt.Errorf("single-statement check: %w", err)
	}

	stmt := parsed.Stmts[0].Stmt

	// If we detect a read-query, do read-query validation.
	if selectStmt := stmt.GetSelectStmt(); selectStmt != nil {
		if err := pp.validateReadQuery(stmt); err != nil {
			return parsing.UndefinedQuery, fmt.Errorf("validating read-query: %w", err)
		}
		return parsing.ReadQuery, nil
	}

	// Otherwise, do a write-query validation.
	if err := pp.validateWriteQuery(stmt); err != nil {
		return parsing.UndefinedQuery, fmt.Errorf("validating write-query: %w", err)
	}

	return parsing.WriteQuery, nil
}

func (pp *QueryValidator) validateWriteQuery(stmt *pg_query.Node) error {
	if err := pp.checkTopLevelUpdateInsertDelete(stmt); err != nil {
		return fmt.Errorf("allowed top level stmt: %w", err)
	}

	if err := pp.checkNoJoinOrSubquery(stmt); err != nil {
		return fmt.Errorf("join or subquery check: %w", err)
	}

	if err := pp.checkNoReturningClause(stmt); err != nil {
		return fmt.Errorf("no returning clause check: %w", err)
	}

	if err := pp.checkNoSystemTablesReferencing(stmt); err != nil {
		return fmt.Errorf("no system-table reference: %w", err)
	}

	if err := pp.checkNonDeterministicFunctions(stmt); err != nil {
		return fmt.Errorf("no non-deterministic func check: %w", err)
	}

	return nil
}

func (pp *QueryValidator) validateReadQuery(node *pg_query.Node) error {
	selectStmt := node.GetSelectStmt()

	if err := pp.checkNoJoinOrSubquery(selectStmt.WhereClause); err != nil {
		return fmt.Errorf("join or subquery in where: %w", err)
	}
	for _, n := range selectStmt.TargetList {
		if err := pp.checkNoJoinOrSubquery(n); err != nil {
			return fmt.Errorf("join or subquery in cols: %w", err)
		}
	}
	for _, n := range selectStmt.FromClause {
		if err := pp.checkNoJoinOrSubquery(n); err != nil {
			return fmt.Errorf("join or subquery in from: %w", err)
		}
	}

	if err := pp.checkNoForUpdateOrShare(selectStmt); err != nil {
		return fmt.Errorf("no for check: %w", err)
	}

	if err := pp.checkNoSystemTablesReferencing(node); err != nil {
		return fmt.Errorf("no system-table referencing check: %w", err)
	}

	return nil
}

func (pp *QueryValidator) checkSingleStatement(parsed *pg_query.ParseResult) error {
	if len(parsed.Stmts) != 1 {
		return &parsing.ErrNoSingleStatement{}
	}
	return nil
}

func (pp *QueryValidator) checkTopLevelUpdateInsertDelete(node *pg_query.Node) error {
	if node.GetUpdateStmt() == nil &&
		node.GetInsertStmt() == nil &&
		node.GetDeleteStmt() == nil {
		return &parsing.ErrNoTopLevelUpdateInsertDelete{}
	}
	return nil
}

func (pp *QueryValidator) checkTopLevelCreate(node *pg_query.Node) error {
	if node.GetCreateStmt() == nil {
		return &parsing.ErrNoTopLevelCreate{}
	}
	return nil
}

func (pp *QueryValidator) checkNoForUpdateOrShare(node *pg_query.SelectStmt) error {
	if node == nil {
		return errEmptyNode
	}

	if len(node.LockingClause) > 0 {
		return &parsing.ErrNoForUpdateOrShare{}
	}
	return nil
}

func (pp *QueryValidator) checkNoReturningClause(node *pg_query.Node) error {
	if node == nil {
		return errEmptyNode
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
		return errUnexpectedNodeType
	}
	return nil
}

func (pp *QueryValidator) checkNoSystemTablesReferencing(node *pg_query.Node) error {
	if node == nil {
		return nil
	}
	if rangeVar := node.GetRangeVar(); rangeVar != nil {
		if strings.HasPrefix(rangeVar.Relname, pp.systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
	} else if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if strings.HasPrefix(insertStmt.Relation.Relname, pp.systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		return pp.checkNoSystemTablesReferencing(insertStmt.SelectStmt)
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		for _, fcn := range selectStmt.FromClause {
			if err := pp.checkNoSystemTablesReferencing(fcn); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if strings.HasPrefix(updateStmt.Relation.Relname, pp.systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := pp.checkNoSystemTablesReferencing(fcn); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if strings.HasPrefix(deleteStmt.Relation.Relname, pp.systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		if err := pp.checkNoSystemTablesReferencing(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := pp.checkNoSystemTablesReferencing(rangeSubselectStmt.Subquery); err != nil {
			return fmt.Errorf("subquery: %w", err)
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := pp.checkNoSystemTablesReferencing(joinExpr.Larg); err != nil {
			return fmt.Errorf("join left arg: %w", err)
		}
		if err := pp.checkNoSystemTablesReferencing(joinExpr.Rarg); err != nil {
			return fmt.Errorf("join right arg: %w", err)
		}
	}
	return nil
}

// checkNonDeterministicFunctions walks the query tree and disallow references to
// functions that aren't deterministic.
func (pp *QueryValidator) checkNonDeterministicFunctions(node *pg_query.Node) error {
	if node == nil {
		return nil
	}
	if sqlValFunc := node.GetSqlvalueFunction(); sqlValFunc != nil {
		return &parsing.ErrNonDeterministicFunction{}
	} else if listStmt := node.GetList(); listStmt != nil {
		for _, item := range listStmt.Items {
			if err := pp.checkNonDeterministicFunctions(item); err != nil {
				return fmt.Errorf("list item: %w", err)
			}
		}
	}
	if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		return pp.checkNonDeterministicFunctions(insertStmt.SelectStmt)
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		for _, nl := range selectStmt.ValuesLists {
			if err := pp.checkNonDeterministicFunctions(nl); err != nil {
				return fmt.Errorf("value list: %w", err)
			}
		}
		for _, fcn := range selectStmt.FromClause {
			if err := pp.checkNonDeterministicFunctions(fcn); err != nil {
				return fmt.Errorf("from: %w", err)
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		for _, t := range updateStmt.TargetList {
			if err := pp.checkNonDeterministicFunctions(t); err != nil {
				return fmt.Errorf("target: %w", err)
			}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := pp.checkNonDeterministicFunctions(fcn); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
		if err := pp.checkNonDeterministicFunctions(updateStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if err := pp.checkNonDeterministicFunctions(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := pp.checkNonDeterministicFunctions(rangeSubselectStmt.Subquery); err != nil {
			return fmt.Errorf("subquery: %w", err)
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := pp.checkNonDeterministicFunctions(joinExpr.Larg); err != nil {
			return fmt.Errorf("join left tree: %w", err)
		}
		if err := pp.checkNonDeterministicFunctions(joinExpr.Rarg); err != nil {
			return fmt.Errorf("join right tree: %w", err)
		}
	} else if aExpr := node.GetAExpr(); aExpr != nil {
		if err := pp.checkNonDeterministicFunctions(aExpr.Lexpr); err != nil {
			return fmt.Errorf("aexpr left: %w", err)
		}
		if err := pp.checkNonDeterministicFunctions(aExpr.Rexpr); err != nil {
			return fmt.Errorf("aexpr right: %w", err)
		}
	} else if resTarget := node.GetResTarget(); resTarget != nil {
		if err := pp.checkNonDeterministicFunctions(resTarget.Val); err != nil {
			return fmt.Errorf("target: %w", err)
		}
	}
	return nil
}

func (pp *QueryValidator) checkNoJoinOrSubquery(node *pg_query.Node) error {
	if node == nil {
		return nil
	}

	if resTarget := node.GetResTarget(); resTarget != nil {
		if err := pp.checkNoJoinOrSubquery(resTarget.Val); err != nil {
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
		if err := pp.checkNoJoinOrSubquery(insertStmt.SelectStmt); err != nil {
			return fmt.Errorf("insert select expr: %w", err)
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if len(updateStmt.FromClause) != 0 {
			return &parsing.ErrJoinOrSubquery{}
		}
		if err := pp.checkNoJoinOrSubquery(updateStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if err := pp.checkNoJoinOrSubquery(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if aExpr := node.GetAExpr(); aExpr != nil {
		if err := pp.checkNoJoinOrSubquery(aExpr.Lexpr); err != nil {
			return fmt.Errorf("aexpr left: %w", err)
		}
		if err := pp.checkNoJoinOrSubquery(aExpr.Rexpr); err != nil {
			return fmt.Errorf("aexpr right: %w", err)
		}
	} else if subLinkExpr := node.GetSubLink(); subLinkExpr != nil {
		return &parsing.ErrJoinOrSubquery{}
	} else if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.Args {
			if err := pp.checkNoJoinOrSubquery(arg); err != nil {
				return fmt.Errorf("bool expr: %w", err)
			}
		}
	}
	return nil
}

func (pp *QueryValidator) checkCreateColTypes(createStmt *pg_query.CreateStmt) error {
	if createStmt == nil {
		return errEmptyNode
	}

	for _, col := range createStmt.TableElts {
		colDef := col.GetColumnDef()
		if colDef == nil {
			return errors.New("unexpected node type in column definition")
		}

	AcceptedTypesFor:
		for _, nameNode := range colDef.TypeName.Names {
			name := nameNode.GetString_()
			if name == nil {
				return fmt.Errorf("unexpected type name node: %v", name)
			}
			// We skip `pg_catalog` since it seems that gets included for some
			// cases of native types.
			if name.Str == "pg_catalog" {
				continue
			}

			for _, typeName := range pp.acceptedTypesNames {
				if name.Str == typeName {
					// The current data type name has a match with accepted
					// types. Continue matching the rest of columns.
					continue AcceptedTypesFor
				}
			}

			return &parsing.ErrInvalidColumnType{ColumnType: name.Str}
		}
	}

	return nil
}
