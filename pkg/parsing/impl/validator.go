package impl

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
	"github.com/textileio/go-tableland/pkg/tables"
)

// QueryValidator enforces the Tablealand SQL spec.
type QueryValidator struct {
	systemTablePrefixes  []string
	createTableNameRegEx *regexp.Regexp
	queryTableNameRegEx  *regexp.Regexp
	config               *parsing.Config
}

var _ parsing.SQLValidator = (*QueryValidator)(nil)

// New returns a Tableland query validator.
func New(systemTablePrefixes []string, opts ...parsing.Option) (parsing.SQLValidator, error) {
	config := parsing.DefaultConfig()
	for _, o := range opts {
		if err := o(config); err != nil {
			return nil, fmt.Errorf("applying provided option: %s", err)
		}
	}

	tablePrefixRegex := "^([A-Za-z]+[A-Za-z0-9_]*)"
	queryTableNameRegEx, _ := regexp.Compile(fmt.Sprintf("%s*_[0-9]+_[0-9]+$", tablePrefixRegex))
	createTableNameRegEx, _ := regexp.Compile(fmt.Sprintf("%s*_[0-9]+$", tablePrefixRegex))

	return &QueryValidator{
		systemTablePrefixes:  systemTablePrefixes,
		createTableNameRegEx: createTableNameRegEx,
		queryTableNameRegEx:  queryTableNameRegEx,
		config:               config,
	}, nil
}

// ValidateCreateTable validates a CREATE TABLE statement.
func (pp *QueryValidator) ValidateCreateTable(query string, chainID tableland.ChainID) (parsing.CreateStmt, error) {
	ast, err := sqlparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the query: %w", err)
	}

	if err := checkNonEmptyStatement(ast); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	stmt := ast.Statements[0]
	if _, ok := stmt.(sqlparser.CreateTableStatement); !ok {
		return nil, &parsing.ErrNoTopLevelCreate{}
	}

	node := stmt.(*sqlparser.CreateTable)
	validTable, err := sqlparser.ValidateCreateTargetTable(node.Table)
	if err != nil {
		return nil, fmt.Errorf("create table name is not valid: %w", err)
	}

	if hasPrefix(validTable.Prefix(), pp.systemTablePrefixes) {
		return nil, &parsing.ErrPrefixTableName{Prefix: validTable.Prefix()}
	}

	if validTable.ChainID() != int64(chainID) {
		return nil, &parsing.ErrInvalidTableName{}
	}

	return &createStmt{
		chainID:       chainID,
		cNode:         node,
		structureHash: node.StructureHash(),
		prefix:        validTable.Prefix(),
	}, nil
}

// ValidateMutatingQuery validates a mutating-query, and a list of mutating statements
// contained in it.
func (pp *QueryValidator) ValidateMutatingQuery(
	query string,
	chainID tableland.ChainID,
) ([]parsing.MutatingStmt, error) {
	if len(query) > pp.config.MaxWriteQuerySize {
		return nil, &parsing.ErrWriteQueryTooLong{
			Length:     len(query),
			MaxAllowed: pp.config.MaxWriteQuerySize,
		}
	}

	ast, err := sqlparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the query: %w", err)
	}

	if err := checkNonEmptyStatement(ast); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	// Since we support write queries with more than one statement,
	// do the write/grant-query validation in each of them. Also, check
	// that each statement reference always the same table.
	var targetTable, refTable *sqlparser.ValidatedTable
	for i := range ast.Statements {
		if ast.Errors[i] != nil {
			return nil, fmt.Errorf("non sysntax error in %d-th statement: %w", i, ast.Errors[i])
		}

		stmt := ast.Statements[i]
		switch s := stmt.(type) {
		case sqlparser.WriteStatement:
			refTable, err = pp.validateWriteQuery(s)
			if err != nil {
				return nil, fmt.Errorf("validating write-query: %w", err)
			}

		case sqlparser.GrantOrRevokeStatement:
			refTable, err = pp.validateGrantQuery(s)
			if err != nil {
				return nil, fmt.Errorf("validating grant-query: %w", err)
			}
		default:
			return nil, &parsing.ErrStatementIsNotSupported{}
		}

		if targetTable == nil {
			targetTable = refTable
		} else if targetTable.Name() != refTable.Name() {
			return nil, &parsing.ErrMultiTableReference{Ref1: targetTable.Name(), Ref2: refTable.Name()}
		}
	}

	if targetTable.ChainID() != int64(chainID) {
		return nil, fmt.Errorf("the query references chain-id %d but expected %d", targetTable.ChainID(), chainID)
	}

	ret := make([]parsing.MutatingStmt, len(ast.Statements))
	for i := range ast.Statements {
		stmt := ast.Statements[i]
		tblID, err := tables.NewTableID(fmt.Sprintf("%d", targetTable.TokenID()))
		if err != nil {
			return nil, &parsing.ErrInvalidTableName{}
		}
		mutatingStmt := &mutatingStmt{
			node:        stmt,
			dbTableName: targetTable.Name(),
			prefix:      targetTable.Prefix(),
			tableID:     tblID,
		}

		switch s := stmt.(type) {
		case sqlparser.WriteStatement:
			if _, ok := s.(*sqlparser.Insert); ok {
				mutatingStmt.operation = tableland.OpInsert
			}
			if _, ok := s.(*sqlparser.Update); ok {
				mutatingStmt.operation = tableland.OpUpdate
			}
			if _, ok := s.(*sqlparser.Delete); ok {
				mutatingStmt.operation = tableland.OpDelete
			}
			ret[i] = &writeStmt{mutatingStmt}
		case sqlparser.GrantOrRevokeStatement:
			if _, ok := s.(*sqlparser.Grant); ok {
				mutatingStmt.operation = tableland.OpGrant
			} else {
				mutatingStmt.operation = tableland.OpRevoke
			}

			ret[i] = &grantStmt{mutatingStmt}
		default:
			return nil, &parsing.ErrStatementIsNotSupported{}
		}
	}

	return ret, nil
}

// ValidateReadQuery validates a read-query, and returns a structured representation of it.
func (pp *QueryValidator) ValidateReadQuery(query string) (parsing.ReadStmt, error) {
	if len(query) > pp.config.MaxReadQuerySize {
		return nil, &parsing.ErrReadQueryTooLong{
			Length:     len(query),
			MaxAllowed: pp.config.MaxReadQuerySize,
		}
	}

	ast, err := sqlparser.Parse(query)
	if err != nil {
		return nil, fmt.Errorf("unable to parse the query: %w", err)
	}

	if err := checkNonEmptyStatement(ast); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	if _, ok := ast.Statements[0].(*sqlparser.Select); !ok {
		return nil, errors.New("the query isn't a read-query")
	}

	return &readStmt{
		statement: ast.Statements[0],
	}, nil
}

type mutatingStmt struct {
	node        sqlparser.Statement
	prefix      string         // From {prefix}_{chainID}_{tableID} -> {prefix}
	tableID     tables.TableID // From {prefix}_{chainID}_{tableID} -> {tableID}
	dbTableName string         // {prefix}_{chainID}_{tableID}
	operation   tableland.Operation
}

var _ parsing.MutatingStmt = (*mutatingStmt)(nil)

func (s *mutatingStmt) GetQuery(resolver sqlparser.WriteStatementResolver) (string, error) {
	if writeStmt, ok := s.node.(sqlparser.WriteStatement); ok {
		query, err := writeStmt.Resolve(resolver)
		if err != nil {
			return "", fmt.Errorf("resolving write statement: %s", err)
		}
		return query, nil
	}

	return s.node.String(), nil
}

func (s *mutatingStmt) GetPrefix() string {
	return s.prefix
}

func (s *mutatingStmt) GetTableID() tables.TableID {
	return s.tableID
}

func (s *mutatingStmt) Operation() tableland.Operation {
	return s.operation
}

func (s *mutatingStmt) GetDBTableName() string {
	return s.dbTableName
}

type writeStmt struct {
	*mutatingStmt
}

var _ parsing.WriteStmt = (*writeStmt)(nil)

func (ws *writeStmt) AddWhereClause(whereClauses string) error {
	// this does not apply to insert
	if ws.Operation() == tableland.OpInsert {
		return parsing.ErrCantAddWhereOnINSERT
	}

	// helper query to extract the where clause from the AST
	helper, err := sqlparser.Parse("UPDATE helper SET foo = 'bar' WHERE " + whereClauses)
	if err != nil {
		return fmt.Errorf("parsing where clauses: %s", err)
	}

	if helper.Errors[0] != nil {
		return fmt.Errorf("parsing where clauses: %s", err)
	}

	whereNode := helper.Statements[0].(*sqlparser.Update).Where
	if updateStmt, ok := ws.node.(*sqlparser.Update); ok {
		updateStmt.AddWhereClause(whereNode)
		return nil
	}

	if deleteStmt, ok := ws.node.(*sqlparser.Delete); ok {
		deleteStmt.AddWhereClause(whereNode)
		return nil
	}

	return nil
}

func (ws *writeStmt) AddReturningClause() error {
	// this does not apply to delete
	if ws.Operation() == tableland.OpDelete {
		return parsing.ErrCantAddReturningOnDELETE
	}

	if ws.Operation() == tableland.OpUpdate {
		updateStmt := ws.node.(*sqlparser.Update)
		updateStmt.ReturningClause = sqlparser.Exprs{&sqlparser.Column{Name: "rowid"}}
		return nil
	}

	if ws.Operation() == tableland.OpInsert {
		insertStmt := ws.node.(*sqlparser.Insert)
		insertStmt.ReturningClause = sqlparser.Exprs{&sqlparser.Column{Name: "rowid"}}
		return nil
	}

	return nil
}

func (ws *writeStmt) CheckColumns(allowedColumns []string) error {
	if ws.Operation() != tableland.OpUpdate {
		return parsing.ErrCanOnlyCheckColumnsOnUPDATE
	}

	allowedColumnsMap := make(map[string]struct{})
	for _, allowedColumn := range allowedColumns {
		allowedColumnsMap[allowedColumn] = struct{}{}
	}

	updateStmt := ws.node.(sqlparser.WriteStatement).(*sqlparser.Update)
	for _, expr := range updateStmt.Exprs {
		if _, ok := allowedColumnsMap[expr.Column.Name.String()]; !ok {
			return fmt.Errorf("column %s is not allowed", expr.Column.Name.String())
		}
	}

	return nil
}

type grantStmt struct {
	*mutatingStmt
}

var _ parsing.GrantStmt = (*grantStmt)(nil)

func (gs *grantStmt) GetRoles() []common.Address {
	grantees := gs.mutatingStmt.node.(sqlparser.GrantOrRevokeStatement).GetRoles()
	roles := make([]common.Address, len(grantees))
	for i, grantee := range grantees {
		roles[i] = common.HexToAddress(grantee)
	}

	return roles
}

func (gs *grantStmt) GetPrivileges() tableland.Privileges {
	privileges := tableland.Privileges{}
	for priv := range gs.mutatingStmt.node.(sqlparser.GrantOrRevokeStatement).GetPrivileges() {
		privilege, _ := tableland.NewPrivilegeFromSQLString(priv)
		privileges = append(privileges, privilege)
	}

	return privileges
}

type readStmt struct {
	statement sqlparser.Statement
}

var _ parsing.ReadStmt = (*readStmt)(nil)

func (s *readStmt) GetQuery(resolver sqlparser.ReadStatementResolver) (string, error) {
	query, err := s.statement.(sqlparser.ReadStatement).Resolve(resolver)
	if err != nil {
		return "", fmt.Errorf("resolving read statement: %s", err)
	}

	return query, nil
}

func (pp *QueryValidator) validateWriteQuery(stmt sqlparser.WriteStatement) (*sqlparser.ValidatedTable, error) {
	if err := checkNoSystemTablesReferencing(stmt, pp.systemTablePrefixes); err != nil {
		return nil, fmt.Errorf("no system-table reference: %w", err)
	}

	insertTable, err := sqlparser.ValidateTargetTable(stmt.GetTable())
	if err != nil {
		return nil, fmt.Errorf("table name is not valid: %w", err)
	}

	if insert, ok := stmt.(*sqlparser.Insert); ok && insert.Select != nil {
		tables, err := sqlparser.ValidateTargetTables(insert.Select)
		if err != nil {
			return nil, fmt.Errorf("validating select table names: %w", err)
		}

		if len(tables) > 1 {
			return nil, fmt.Errorf("select should have only one table")
		}

		if tables[0].ChainID() != insertTable.ChainID() {
			return nil, &parsing.ErrInsertWithSelectChainMistmatch{
				InsertChainID: insertTable.ChainID(),
				SelectChainID: tables[0].ChainID(),
			}
		}
	}

	return insertTable, nil
}

func (pp *QueryValidator) validateGrantQuery(stmt sqlparser.GrantOrRevokeStatement) (*sqlparser.ValidatedTable, error) {
	// check if roles are ETH addresses
	for _, role := range stmt.GetRoles() {
		addr := common.Address{}
		if err := addr.UnmarshalText([]byte(role)); err != nil {
			return nil, &parsing.ErrRoleIsNotAnEthAddress{}
		}
	}

	table, err := sqlparser.ValidateTargetTable(stmt.GetTable())
	if err != nil {
		return nil, fmt.Errorf("table name is not valid: %w", err)
	}

	return table, nil
}

func checkNonEmptyStatement(parsed *sqlparser.AST) error {
	if len(parsed.Statements) == 0 {
		return &parsing.ErrEmptyStatement{}
	}
	return nil
}

func checkNoSystemTablesReferencing(stmt sqlparser.WriteStatement, systemTablePrefixes []string) error {
	if hasPrefix(stmt.GetTable().String(), systemTablePrefixes) {
		return &parsing.ErrSystemTableReferencing{}
	}

	return nil
}

func hasPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	return false
}

type createStmt struct {
	chainID       tableland.ChainID
	cNode         *sqlparser.CreateTable
	structureHash string
	prefix        string
}

var _ parsing.CreateStmt = (*createStmt)(nil)

func (cs *createStmt) GetRawQueryForTableID(id tables.TableID) (string, error) {
	cs.cNode.Table.Name = sqlparser.Identifier(fmt.Sprintf("%s_%d_%s", cs.prefix, cs.chainID, id))
	cs.cNode.StrictMode = true
	return cs.cNode.String(), nil
}

func (cs *createStmt) GetStructureHash() string {
	return cs.structureHash
}

func (cs *createStmt) GetPrefix() string {
	return cs.prefix
}
