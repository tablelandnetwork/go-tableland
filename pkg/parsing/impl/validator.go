package impl

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
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

	tablePrefixRegex := "([A-Za-z]+[A-Za-z0-9_]*)"
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
		return nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := checkNonEmptyStatement(ast); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	stmt := ast.Statements[0]
	if _, ok := stmt.(sqlparser.CreateTableStatement); !ok {
		return nil, &parsing.ErrNoTopLevelCreate{}
	}

	if ast.Errors[0] != nil {
		return nil, fmt.Errorf("non syntax error: %w", ast.Errors[0])
	}

	node := stmt.(*sqlparser.CreateTable)
	if !pp.createTableNameRegEx.MatchString(node.Table.String()) {
		return nil, &parsing.ErrInvalidTableName{}
	}
	parts := strings.Split(node.Table.String(), "_")
	if len(parts) < 2 {
		return nil, fmt.Errorf("table name isn't referencing the chain id")
	}

	prefix := strings.Join(parts[:len(parts)-1], "_")
	if hasPrefix(prefix, pp.systemTablePrefixes) {
		return nil, &parsing.ErrInvalidTableName{}
	}

	strChainID := parts[len(parts)-1]
	tableChainID, err := strconv.ParseInt(strChainID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("parsing chain id in table name: %s", err)
	}
	if tableChainID != int64(chainID) {
		return nil, &parsing.ErrInvalidTableName{}
	}

	return &createStmt{
		chainID:       chainID,
		cNode:         node,
		structureHash: node.StructureHash(),
		prefix:        prefix,
	}, nil
}

// ValidateMutatingQuery validates a mutating-query, and a list of mutating statements
// contained in it.
func (pp *QueryValidator) ValidateMutatingQuery(
	query string,
	chainID tableland.ChainID) ([]parsing.MutatingStmt, error) {
	if len(query) > pp.config.MaxWriteQuerySize {
		return nil, &parsing.ErrWriteQueryTooLong{
			Length:     len(query),
			MaxAllowed: pp.config.MaxWriteQuerySize,
		}
	}

	ast, err := sqlparser.Parse(query)
	if err != nil {
		return nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := checkNonEmptyStatement(ast); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	// Since we support write queries with more than one statement,
	// do the write/grant-query validation in each of them. Also, check
	// that each statement reference always the same table.
	var targetTable, refTable string
	for i := range ast.Statements {
		if ast.Errors[i] != nil {
			return nil, fmt.Errorf("non syntax error: %w", ast.Errors[i])
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

		if targetTable == "" {
			targetTable = refTable
		} else if targetTable != refTable {
			return nil, &parsing.ErrMultiTableReference{Ref1: targetTable, Ref2: refTable}
		}
	}

	prefix, queryChainID, tableID, err := pp.deconstructRefTable(targetTable)
	if err != nil {
		return nil, fmt.Errorf("deconstructing referenced table name: %w", err)
	}
	if queryChainID != chainID {
		return nil, fmt.Errorf("the query references chain-id %d but expected %d", queryChainID, chainID)
	}

	ret := make([]parsing.MutatingStmt, len(ast.Statements))
	for i := range ast.Statements {
		stmt := ast.Statements[i]
		tblID, err := tableland.NewTableID(tableID)
		if err != nil {
			return nil, &parsing.ErrInvalidTableName{}
		}
		mutatingStmt := &mutatingStmt{
			node:        stmt,
			dbTableName: targetTable,
			prefix:      prefix,
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
		return nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := checkNonEmptyStatement(ast); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	if _, ok := ast.Statements[0].(*sqlparser.Select); !ok {
		return nil, errors.New("the query isn't a read-query")
	}

	if ast.Errors[0] != nil {
		return nil, fmt.Errorf("non syntax error: %w", ast.Errors[0])
	}

	return &readStmt{
		query: query,
	}, nil
}

// deconstructRefTable returns three main deconstructions of the reference table name in the query:
// 1) The {prefix}  of {prefix}_{chainID}_{ID}, if any.
// 2) The {chainID} of {prefix}_{chainID}_{ID}.
// 3) The {tableID} of {prefix}_{chainID}_{ID}.
// If the referenced table is a system table, it will only return.
func (pp *QueryValidator) deconstructRefTable(refTable string) (string, tableland.ChainID, string, error) {
	if hasPrefix(refTable, pp.systemTablePrefixes) {
		return "", 0, refTable, nil
	}
	if !pp.queryTableNameRegEx.MatchString(refTable) {
		return "", 0, "", &parsing.ErrInvalidTableName{}
	}

	parts := strings.Split(refTable, "_")
	// This validation is redundant considering that refTable matches the expected regex,
	// so we're being a bit paranoid here since it's an easy check.
	if len(parts) < 2 {
		return "", 0, "", &parsing.ErrInvalidTableName{}
	}

	tableID := parts[len(parts)-1]
	partChainID := parts[len(parts)-2]
	prefix := strings.Join(parts[:len(parts)-2], "_")

	chainID, err := strconv.ParseInt(partChainID, 10, 64)
	if err != nil {
		return "", 0, "", &parsing.ErrInvalidTableName{}
	}

	return prefix, tableland.ChainID(chainID), tableID, nil
}

type mutatingStmt struct {
	node        sqlparser.Statement
	prefix      string            // From {prefix}_{chainID}_{tableID} -> {prefix}
	tableID     tableland.TableID // From {prefix}_{chainID}_{tableID} -> {tableID}
	dbTableName string            // {prefix}_{chainID}_{tableID}
	operation   tableland.Operation
}

var _ parsing.MutatingStmt = (*mutatingStmt)(nil)

func (s *mutatingStmt) GetQuery() (string, error) {
	return s.node.String(), nil
}

func (s *mutatingStmt) GetPrefix() string {
	return s.prefix
}

func (s *mutatingStmt) GetTableID() tableland.TableID {
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

	whereNode := helper.Statements[0].(sqlparser.WriteStatement).(*sqlparser.Update).Where
	if updateStmt, ok := ws.node.(sqlparser.WriteStatement).(*sqlparser.Update); ok {
		updateStmt.AddWhereClause(whereNode)
		return nil
	}

	if deleteStmt, ok := ws.node.(sqlparser.WriteStatement).(*sqlparser.Delete); ok {
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
		updateStmt := ws.node.(sqlparser.WriteStatement).(*sqlparser.Update)
		updateStmt.ReturningClause = sqlparser.Exprs{&sqlparser.Column{Name: "rowid"}}
		return nil
	}

	if ws.Operation() == tableland.OpInsert {
		insertStmt := ws.node.(sqlparser.WriteStatement).(*sqlparser.Insert)
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
	query string
}

var _ parsing.ReadStmt = (*readStmt)(nil)

func (s *readStmt) GetQuery() (string, error) {
	return s.query, nil
}

func (pp *QueryValidator) validateWriteQuery(stmt sqlparser.WriteStatement) (string, error) {
	if err := checkNoSystemTablesReferencing(stmt, pp.systemTablePrefixes); err != nil {
		return "", fmt.Errorf("no system-table reference: %w", err)
	}

	return stmt.GetTable().String(), nil
}

func (pp *QueryValidator) validateGrantQuery(stmt sqlparser.GrantOrRevokeStatement) (string, error) {
	// check if roles are ETH addresses
	for _, role := range stmt.GetRoles() {
		addr := common.Address{}
		if err := addr.UnmarshalText([]byte(role)); err != nil {
			return "", &parsing.ErrRoleIsNotAnEthAddress{}
		}
	}

	return stmt.GetTable().String(), nil
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

func (cs *createStmt) GetRawQueryForTableID(id tableland.TableID) (string, error) {
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
