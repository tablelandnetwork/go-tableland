package impl

import (
	"fmt"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/internal/parsing"
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

	/*
		stmt := parsed.Stmts[0].Stmt
		if err := pp.checkTopLevelUpdateInsertDelete(stmtl); err != nil {
			return err
		}
	*/

	//if err := pp.checkNoReturningClause(); err != nil {
	//	return err
	//}

	// DELETEs //
	// TODO: disallow RETURNING in UPDATE.
	// TODO: disallow internal tables referencing.
	// TODO: disallow non-deterministic.

	// UPDATEs //
	// TODO: disallow RETURNING in UPDATE.
	// TODO: disallow internal tables referencing.
	// TODO: disallow non-deterministic.

	// INSERTs //
	// TODO: disallow RETURNING in INSERT
	// TODO: disallow internal tables referencing.
	// TODO: disallow non-deterministic.

	insertStmt := parsed.Stmts[0].Stmt.GetInsertStmt()
	if err := pp.checkNoNonDeterministicFunctions(insertStmt); err != nil {
		return err
	}
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

	if err := pp.checkNoSystemTablesReferencing(selectStmt.FromClause); err != nil {
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
		return fmt.Errorf("invalid select statement node")
	}

	if len(node.LockingClause) > 0 {
		return &parsing.ErrNoForUpdateOrShare{}
	}
	return nil
}

func (pp *PostgresParser) checkNoSystemTablesReferencing(fromClauseNodes []*pg_query.Node) error {
	for _, fcn := range fromClauseNodes {
		// 1. If is referencing a direct table, do the prefix check.
		rv := fcn.GetRangeVar()
		if rv != nil {
			if strings.HasPrefix(rv.Relname, pp.systemTablePrefix) {
				return &parsing.ErrSystemTableReferencing{}
			}
			continue
		}

		// 2. If isn't a referencing a direct table, do a recursive check.
		//    i.e: look for sytem tables references in nested SELECTs in FROMs.
		selectStmt := fcn.GetSelectStmt()
		if selectStmt == nil {
			return &parsing.ErrSystemTableReferencing{
				ParsingError: "FROM clause isn't a SELECT",
			}
		}
		return pp.checkNoSystemTablesReferencing(selectStmt.FromClause)
	}

	return nil
}

func (pp *PostgresParser) checkNoNonDeterministicFunctions(node *pg_query.InsertStmt) error {
	if node == nil {
		return fmt.Errorf("invalid insert statement node")
	}
	sel := node.SelectStmt.GetSelectStmt()

	for _, i := range sel.ValuesLists[0].GetList().Items {
		fmt.Printf("item: %v\n", i)
	}

	return nil
}
