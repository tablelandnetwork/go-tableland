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

	panic("TODO")
}

func (pp *PostgresParser) ValidateRunSQL(query string) error {
	panic("TODO")

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

	if err := pp.checkNoSystemTablesReferencing(selectStmt); err != nil {
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

func (pp *PostgresParser) checkNoSystemTablesReferencing(node *pg_query.SelectStmt) error {
	if node == nil {
		return fmt.Errorf("invalid select statement node")
	}

	for _, fc := range node.FromClause {
		rv := fc.GetRangeVar()
		if rv == nil {
			// TODO(jsign): return specific error and add tests.
			return fmt.Errorf("not allowed from clause")
		}
		if strings.HasPrefix(rv.Relname, pp.systemTablePrefix) {
			// TODO(jsign): return specific error and add tests.
			return fmt.Errorf("not allowed system table")
		}
	}

	return nil
}
