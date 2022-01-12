package impl

import (
	"errors"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/internal/parsing"
)

var (
	errEmptyNode          = errors.New("empty node")
	errUnexpectedNodeType = errors.New("unexpected node type")
)

type PostgresParser struct {
	systemTablePrefix string
}

var _ parsing.Parser = (*PostgresParser)(nil)

func New(systemTablePrefix string) *PostgresParser {
	return &PostgresParser{
		systemTablePrefix: systemTablePrefix,
	}
}

func (pp *PostgresParser) ValidateCreateTable(query string) error {
	// TODO: only allow single statement.
	// TODO: only allow CREATE
	// TODO: not allow table names with systemTablePrefix

	panic("TODO")
}

func (pp *PostgresParser) ValidateRunSQL(query string) error {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := pp.checkSingleStatement(parsed); err != nil {
		return err
	}

	stmt := parsed.Stmts[0].Stmt
	if err := pp.checkTopLevelUpdateInsertDelete(stmt); err != nil {
		return err
	}

	if err := pp.checkNoReturningClause(stmt); err != nil {
		return err
	}

	if err := pp.checkNoSystemTablesReferencing(stmt); err != nil {
		return err
	}

	// TODO: disallow non-deterministic.

	return nil
}

func (pp *PostgresParser) ValidateReadQuery(query string) error {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := pp.checkSingleStatement(parsed); err != nil {
		return err
	}

	stmt := parsed.Stmts[0].Stmt
	if err := pp.checkTopLevelSelect(stmt); err != nil {
		return err
	}

	selectStmt := stmt.GetSelectStmt()
	if err := pp.checkNoForUpdateOrShare(selectStmt); err != nil {
		return err
	}

	if err := pp.checkNoSystemTablesReferencing(stmt); err != nil {
		return err
	}

	return nil
}

func (pp *PostgresParser) checkSingleStatement(parsed *pg_query.ParseResult) error {
	if len(parsed.Stmts) != 1 {
		return &parsing.ErrNoSingleStatement{}
	}
	return nil
}

func (pp *PostgresParser) checkTopLevelSelect(node *pg_query.Node) error {
	if node.GetSelectStmt() == nil {
		return &parsing.ErrNoTopLevelSelect{}
	}
	return nil
}

func (pp *PostgresParser) checkTopLevelUpdateInsertDelete(node *pg_query.Node) error {
	if node.GetUpdateStmt() == nil &&
		node.GetInsertStmt() == nil &&
		node.GetDeleteStmt() == nil {
		return &parsing.ErrNoTopLevelUpdateInsertDelete{}
	}
	return nil
}

func (pp *PostgresParser) checkNoForUpdateOrShare(node *pg_query.SelectStmt) error {
	if node == nil {
		return errEmptyNode
	}

	if len(node.LockingClause) > 0 {
		return &parsing.ErrNoForUpdateOrShare{}
	}
	return nil
}

func (pp *PostgresParser) checkNoReturningClause(node *pg_query.Node) error {
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

func (pp *PostgresParser) checkNoSystemTablesReferencing(node *pg_query.Node) error {
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
				return err
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if strings.HasPrefix(updateStmt.Relation.Relname, pp.systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := pp.checkNoSystemTablesReferencing(fcn); err != nil {
				return err
			}
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if strings.HasPrefix(deleteStmt.Relation.Relname, pp.systemTablePrefix) {
			return &parsing.ErrSystemTableReferencing{}
		}
		if err := pp.checkNoSystemTablesReferencing(deleteStmt.WhereClause); err != nil {
			return err
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := pp.checkNoSystemTablesReferencing(rangeSubselectStmt.Subquery); err != nil {
			return err
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := pp.checkNoSystemTablesReferencing(joinExpr.Larg); err != nil {
			return err
		}
		if err := pp.checkNoSystemTablesReferencing(joinExpr.Rarg); err != nil {
			return err
		}
	}
	return nil
}
