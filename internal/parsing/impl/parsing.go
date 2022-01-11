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

	// TODO: switch INSERT and UPDATE

	// UPDATEs //
	// TODO: disallow SELECTs in UPDATE froms
	// TODO: disallow RETURNING in UPDATE.
	// TODO: disallow internal tables referencing.
	// TODO: disallow non-deterministic.

	// INSERTs //
	// TODO: disallow SELECT in INSERTs
	// TODO: disallow RETURNING in INSERT
	// TODO: disallow internal tables referencing.
	// TODO: disallow non-deterministic funcs.
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

	if err := pp.checkTopLevelSelect(parsed.Stmts[0].Stmt); err != nil {
		return err
	}

	selectStmt := parsed.Stmts[0].Stmt.GetSelectStmt()
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
