package impl

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/internal/parsing"
)

type PostgresParser struct {
}

var _ parsing.Parser = (*PostgresParser)(nil)

func New() *PostgresParser {
	return &PostgresParser{}
}

func (pp *PostgresParser) ValidateCreateTable(query string) error {

	panic("TODO")
}

func (pp *PostgresParser) ValidateRunSQL(query string) error {
	panic("not implemented") // TODO: Implement
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

	if err := pp.checkNoForUpdateOrShare(parsed.Stmts[0].Stmt.GetSelectStmt()); err != nil {
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
