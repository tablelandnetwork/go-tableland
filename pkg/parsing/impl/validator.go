package impl

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	pg_query "github.com/pganalyze/pg_query_go/v2"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/parsing"
)

var (
	errEmptyNode          = errors.New("empty node")
	errUnexpectedNodeType = errors.New("unexpected node type")
)

// QueryValidator enforces PostgresSQL constraints for Tableland.
type QueryValidator struct {
	chainID             tableland.ChainID
	systemTablePrefixes []string
	acceptedTypesNames  []string
	rawTablenameRegEx   *regexp.Regexp
	maxAllowedColumns   int
	maxTextLength       int
}

var _ parsing.SQLValidator = (*QueryValidator)(nil)

// New returns a Tableland query validator.
func New(
	systemTablePrefixes []string,
	chainID tableland.ChainID,
	maxAllowedColumns int,
	maxTextLength int) *QueryValidator {
	// We create here a flattened slice of all the accepted type names from
	// the parsing.AcceptedTypes source of truth. We do this since having a
	// slice is easier and faster to do checks.
	var acceptedTypesNames []string
	for _, at := range parsing.AcceptedTypes {
		acceptedTypesNames = append(acceptedTypesNames, at.Names...)
	}

	rawTablenameRegEx, _ := regexp.Compile(`^\w*_[0-9]+$`)

	return &QueryValidator{
		systemTablePrefixes: systemTablePrefixes,
		acceptedTypesNames:  acceptedTypesNames,
		rawTablenameRegEx:   rawTablenameRegEx,
		maxAllowedColumns:   maxAllowedColumns,
		maxTextLength:       maxTextLength,
		chainID:             chainID,
	}
}

// ValidateCreateTable validates the provided query and returns an error
// if the CREATE statement isn't allowed. Returns nil otherwise.
func (pp *QueryValidator) ValidateCreateTable(query string) (parsing.CreateStmt, error) {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := checkNonEmptyStatement(parsed); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	if err := checkSingleStatement(parsed); err != nil {
		return nil, fmt.Errorf("single-statement check: %w", err)
	}

	stmt := parsed.Stmts[0].Stmt
	if err := checkTopLevelCreate(stmt); err != nil {
		return nil, fmt.Errorf("allowed top level stmt: %w", err)
	}

	colNameTypes, err := checkCreateColTypes(stmt.GetCreateStmt(), pp.acceptedTypesNames)
	if err != nil {
		return nil, fmt.Errorf("disallowed column types: %w", err)
	}

	if pp.maxAllowedColumns > 0 && len(colNameTypes) > pp.maxAllowedColumns {
		return nil, &parsing.ErrTooManyColumns{
			ColumnCount: len(colNameTypes),
			MaxAllowed:  pp.maxAllowedColumns,
		}
	}

	return genCreateStmt(stmt, colNameTypes, pp.chainID), nil
}

// ValidateRunSQL validates the query and returns an error if isn't allowed.
// If the query validates correctly, it returns the query type and nil.
func (pp *QueryValidator) ValidateRunSQL(query string) (parsing.SugaredReadStmt, []parsing.SugaredMutatingStmt, error) {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return nil, nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := checkNonEmptyStatement(parsed); err != nil {
		return nil, nil, fmt.Errorf("empty-statement check: %w", err)
	}

	stmt := parsed.Stmts[0].Stmt

	if selectStmt := stmt.GetSelectStmt(); selectStmt != nil {
		if err := checkSingleStatement(parsed); err != nil {
			return nil, nil, fmt.Errorf("single-statement check: %w", err)
		}
		if err := validateReadQuery(stmt); err != nil {
			return nil, nil, fmt.Errorf("validating read-query: %w", err)
		}
		return &sugaredReadStmt{
			pp:   pp,
			node: stmt,
		}, nil, nil
	}

	// It's a write-query or a grant-query.

	// Since we support write queries with more than one statement,
	// do the write/grant-query validation in each of them. Also, check
	// that each statement reference always the same table.
	var targetTable, refTable string
	for i := range parsed.Stmts {
		stmt := parsed.Stmts[i].Stmt

		switch {
		case isWrite(stmt):
			refTable, err = pp.validateWriteQuery(stmt)
			if err != nil {
				return nil, nil, fmt.Errorf("validating write-query: %w", err)
			}
		case isGrant(stmt):
			refTable, err = pp.validateGrantQuery(stmt)
			if err != nil {
				return nil, nil, fmt.Errorf("validating grant-query: %w", err)
			}
		default:
			return nil, nil, &parsing.ErrStatementIsNotSupported{}
		}

		if targetTable == "" {
			targetTable = refTable
		} else if targetTable != refTable {
			return nil, nil, &parsing.ErrMultiTableReference{Ref1: targetTable, Ref2: refTable}
		}
	}

	namePrefix, tableName, dbTableName, err := pp.deconstructRefTable(targetTable)
	if err != nil {
		return nil, nil, fmt.Errorf("deconstructing referenced table name: %w", err)
	}

	ret := make([]parsing.SugaredMutatingStmt, len(parsed.Stmts))
	for i := range parsed.Stmts {
		stmt := parsed.Stmts[i].Stmt
		s := &sugaredMutatingStmt{
			dbTableName: dbTableName,
			node:        stmt,
			namePrefix:  namePrefix,
			tableName:   tableName,
		}

		switch {
		case isWrite(stmt):
			if stmt.GetInsertStmt() != nil {
				s.operation = tableland.OpInsert
			}
			if stmt.GetUpdateStmt() != nil {
				s.operation = tableland.OpUpdate
			}
			if stmt.GetDeleteStmt() != nil {
				s.operation = tableland.OpDelete
			}
			ret[i] = &sugaredWriteStmt{s}
		case isGrant(stmt):
			if stmt.GetGrantStmt().IsGrant {
				s.operation = tableland.OpGrant
			} else {
				s.operation = tableland.OpRevoke
			}

			ret[i] = &sugaredGrantStmt{s}
		default:
			return nil, nil, &parsing.ErrStatementIsNotSupported{}
		}
	}

	return nil, ret, nil
}

// deconstructRefTable returns three main deconstructions of the reference table name in the query:
// 1) The {name} of {name}_{ID}
// 2) The _{ID} of {name}_{ID}
// 3) A chain-scoped corresponding real name of the table, from {name}_{ID} -> _{chanID}_{ID}, which will be used
//    for desugaring the query.
// It covers the special case of the query referencing system tables, and acting accordingly.
func (pp *QueryValidator) deconstructRefTable(refTable string) (string, string, string, error) {
	if hasPrefix(refTable, pp.systemTablePrefixes) {
		return "", refTable, refTable, nil
	}
	if !pp.rawTablenameRegEx.MatchString(refTable) {
		return "", "", "", &parsing.ErrInvalidTableName{}
	}

	var namePrefix, tableID string
	sepIdx := strings.LastIndex(refTable, "_")
	if sepIdx == -1 {
		tableID = refTable // No name prefix case, _{ID}.
	} else {
		namePrefix = refTable[:sepIdx] // If sepIdx==0, this is correct too.
		tableID = refTable[sepIdx:]    // {name}_{id} -> _{id}
	}
	dbTableName := fmt.Sprintf("_%d%s", pp.chainID, tableID)

	return namePrefix, tableID, dbTableName, nil
}

type sugaredMutatingStmt struct {
	node        *pg_query.Node
	namePrefix  string // From {name}_{tableID} -> {name}
	tableName   string // From {name}_{ID} -> _{ID}
	dbTableName string // From {name}_{ID} -> _{chainID}_{ID}
	operation   tableland.Operation
}

func (s *sugaredMutatingStmt) GetDesugaredQuery() (string, error) {
	parsedTree := &pg_query.ParseResult{}

	if insertStmt := s.node.GetInsertStmt(); insertStmt != nil {
		insertStmt.Relation.Relname = s.dbTableName
	} else if updateStmt := s.node.GetUpdateStmt(); updateStmt != nil {
		updateStmt.Relation.Relname = s.dbTableName
	} else if deleteStmt := s.node.GetDeleteStmt(); deleteStmt != nil {
		deleteStmt.Relation.Relname = s.dbTableName
	} else if grantStmt := s.node.GetGrantStmt(); grantStmt != nil {
		// It is safe to assume Objects has always one element.
		// This is validated in the checkPrivileges call.

		grantStmt.Objects[0].GetRangeVar().Relname = s.dbTableName
	}
	rs := &pg_query.RawStmt{Stmt: s.node}
	parsedTree.Stmts = []*pg_query.RawStmt{rs}
	wq, err := pg_query.Deparse(parsedTree)
	if err != nil {
		return "", fmt.Errorf("deparsing statement: %s", err)
	}
	return wq, nil
}

func (s *sugaredMutatingStmt) GetNamePrefix() string {
	return s.namePrefix
}

func (s *sugaredMutatingStmt) GetTableID() tableland.TableID {
	tid, _ := tableland.NewTableID(s.tableName[1:])
	return tid
}

func (s *sugaredMutatingStmt) Operation() tableland.Operation {
	return s.operation
}

func (ws *sugaredMutatingStmt) GetDBTableName() string {
	return ws.dbTableName
}

type sugaredWriteStmt struct {
	*sugaredMutatingStmt
}

func (ws *sugaredWriteStmt) AddWhereClause(whereClauses string) error {
	// this does not apply to insert
	if ws.Operation() == tableland.OpInsert {
		return parsing.ErrCantAddWhereOnINSERT
	}

	// helper query to extract the where clause from the AST
	helper, err := pg_query.Parse("UPDATE helper SET foo = 'bar' WHERE " + whereClauses)
	if err != nil {
		return fmt.Errorf("parsing where clauses: %s", err)
	}

	updateStmt := ws.node.GetUpdateStmt()
	var newWhereClause *pg_query.Node
	if updateStmt.GetWhereClause() != nil {
		// merge both where clauses nodes
		boolExpr := &pg_query.BoolExpr{
			Boolop: 1,
			Args: []*pg_query.Node{
				updateStmt.GetWhereClause(),
				helper.Stmts[0].GetStmt().GetUpdateStmt().GetWhereClause(),
			},
		}

		newWhereClause = &pg_query.Node{Node: &pg_query.Node_BoolExpr{BoolExpr: boolExpr}}
	} else {
		// use the where clause from the helper query, because the stmt had none
		newWhereClause = helper.Stmts[0].GetStmt().GetUpdateStmt().GetWhereClause()
	}

	updateStmt.WhereClause = newWhereClause
	return nil
}

func (ws *sugaredWriteStmt) AddReturningClause() error {
	// this does not apply to delete
	if ws.Operation() == tableland.OpDelete {
		return parsing.ErrCantAddReturningOnDELETE
	}

	ctidString := &pg_query.Node_String_{
		String_: &pg_query.String{Str: "ctid"},
	}

	columnRef := &pg_query.Node{Node: &pg_query.Node_ColumnRef{
		ColumnRef: &pg_query.ColumnRef{
			Fields: []*pg_query.Node{{Node: ctidString}},
		},
	}}

	resTarget := &pg_query.Node_ResTarget{
		ResTarget: &pg_query.ResTarget{
			Val: columnRef,
		},
	}

	returningLists := []*pg_query.Node{{Node: resTarget}}

	if ws.Operation() == tableland.OpUpdate {
		updateStmt := ws.node.GetUpdateStmt()
		updateStmt.ReturningList = returningLists
		return nil
	}

	if ws.Operation() == tableland.OpInsert {
		insertStmt := ws.node.GetInsertStmt()
		insertStmt.ReturningList = returningLists
		return nil
	}

	return nil
}

func (ws *sugaredWriteStmt) CheckColumns(allowedColumns []string) error {
	if ws.Operation() != tableland.OpUpdate {
		return parsing.ErrCanOnlyCheckColumnsOnUPDATE
	}

	allowedColumnsMap := make(map[string]struct{})
	for _, allowedColumn := range allowedColumns {
		allowedColumnsMap[allowedColumn] = struct{}{}
	}

	updateStmt := ws.node.GetUpdateStmt()

	for _, target := range updateStmt.TargetList {
		resTarget := target.GetResTarget()
		if resTarget == nil {
			continue
		}

		if _, ok := allowedColumnsMap[resTarget.Name]; !ok {
			return fmt.Errorf("column %s is not allowed", resTarget.Name)
		}
	}

	return nil
}

type sugaredGrantStmt struct {
	*sugaredMutatingStmt
}

func (gs *sugaredGrantStmt) GetRoles() []common.Address {
	// The rolenames of grantees are safe to use.
	// They were already validated in the previous checkRoles call.

	grantees := gs.sugaredMutatingStmt.node.GetGrantStmt().GetGrantees()
	roles := make([]common.Address, len(grantees))
	for i, grantee := range grantees {
		roles[i] = common.HexToAddress(grantee.GetRoleSpec().GetRolename())
	}

	return roles
}

func (gs *sugaredGrantStmt) GetPrivileges() tableland.Privileges {
	privilegesNodes := gs.sugaredMutatingStmt.node.GetGrantStmt().GetPrivileges()
	privileges := make(tableland.Privileges, len(privilegesNodes))
	for i, privilegeNode := range privilegesNodes {
		// error is safe to ignore here because the SQL strings were validated before
		// in checkPrivileges(...) method.
		privilege, _ := tableland.NewPrivilegeFromSQLString(privilegeNode.GetAccessPriv().GetPrivName())
		privileges[i] = privilege
	}

	return privileges
}

type sugaredReadStmt struct {
	pp   *QueryValidator
	node *pg_query.Node
}

func (s *sugaredReadStmt) GetDesugaredQuery() (string, error) {
	parsedTree := &pg_query.ParseResult{}

	if selectStmt := s.node.GetSelectStmt(); selectStmt != nil {
		if err := s.pp.deepSelectDesugaring(s.node); err != nil {
			return "", fmt.Errorf("deep desugaring: %s", err)
		}
	}

	rs := &pg_query.RawStmt{Stmt: s.node}
	parsedTree.Stmts = []*pg_query.RawStmt{rs}
	wq, err := pg_query.Deparse(parsedTree)
	if err != nil {
		return "", fmt.Errorf("deparsing statement: %s", err)
	}
	return wq, nil
}

func (pp *QueryValidator) deepSelectDesugaring(stmt *pg_query.Node) error {
	if stmt == nil {
		return nil
	}

	if selectStmt := stmt.GetSelectStmt(); selectStmt != nil {
		//buf, _ := json.MarshalIndent(selectStmt, "", "  ")
		//fmt.Printf("SELECT: %s\n", buf)
		for _, col := range selectStmt.TargetList {
			if err := pp.deepSelectDesugaring(col); err != nil {
				return fmt.Errorf("desugaring column: %s", err)
			}
		}
		for _, from := range selectStmt.FromClause {
			if err := pp.deepSelectDesugaring(from); err != nil {
				return fmt.Errorf("desugaring from: %s", err)
			}
		}
		if err := pp.deepSelectDesugaring(selectStmt.WhereClause); err != nil {
			return fmt.Errorf("desugaring where: %s", err)
		}
	} else if rangeVar := stmt.GetRangeVar(); rangeVar != nil {
		if rangeVar.Relname != "" {
			_, _, dbName, err := pp.deconstructRefTable(rangeVar.Relname)
			if err != nil {
				return fmt.Errorf("deconstructing table name %s: %s", rangeVar.Relname, err)
			}
			rangeVar.Relname = dbName
		}
	} else if resTarget := stmt.GetResTarget(); resTarget != nil {
		if err := pp.deepSelectDesugaring(resTarget.GetVal()); err != nil {
			return fmt.Errorf("desugaring res target: %s", err)
		}
	} else if joinExpr := stmt.GetJoinExpr(); joinExpr != nil {
		if err := pp.deepSelectDesugaring(joinExpr.Larg); err != nil {
			return fmt.Errorf("desugaring larg of join expr: %s", err)
		}
		if err := pp.deepSelectDesugaring(joinExpr.Rarg); err != nil {
			return fmt.Errorf("desugaring rarg of join expr: %s", err)
		}
	} else if subLink := stmt.GetSubLink(); subLink != nil {
		if err := pp.deepSelectDesugaring(subLink.Subselect); err != nil {
			return fmt.Errorf("desugaring sublink subselect: %s", err)
		}
	} else if funcCall := stmt.GetFuncCall(); funcCall != nil {
		for _, arg := range funcCall.Args {
			if err := pp.deepSelectDesugaring(arg); err != nil {
				return fmt.Errorf("desugaring func arg: %s", err)
			}
		}
	}

	return nil
}

func (pp *QueryValidator) validateWriteQuery(stmt *pg_query.Node) (string, error) {
	if err := checkTopLevelUpdateInsertDelete(stmt); err != nil {
		return "", fmt.Errorf("allowed top level stmt: %w", err)
	}

	if err := checkNoJoinOrSubquery(stmt); err != nil {
		return "", fmt.Errorf("join or subquery check: %w", err)
	}

	if err := checkNoReturningClause(stmt); err != nil {
		return "", fmt.Errorf("no returning clause check: %w", err)
	}

	if err := checkNoRelationAlias(stmt); err != nil {
		return "", fmt.Errorf("no relation alias check: %w", err)
	}

	if err := checkNoSystemTablesReferencing(stmt, pp.systemTablePrefixes); err != nil {
		return "", fmt.Errorf("no system-table reference: %w", err)
	}

	if err := checkNonDeterministicFunctions(stmt); err != nil {
		return "", fmt.Errorf("no non-deterministic func check: %w", err)
	}

	if err := checkMaxTextValueLength(stmt, pp.maxTextLength); err != nil {
		return "", fmt.Errorf("max text length check: %w", err)
	}

	referencedTable, err := getReferencedTable(stmt)
	if err != nil {
		return "", fmt.Errorf("get referenced table: %w", err)
	}

	return referencedTable, nil
}

func (pp *QueryValidator) validateGrantQuery(stmt *pg_query.Node) (string, error) {
	if err := checkTopLevelGrant(stmt); err != nil {
		return "", fmt.Errorf("allowed top level stmt: %w", err)
	}

	grantStmt := stmt.GetGrantStmt()
	if err := checkGrantType(grantStmt); err != nil {
		return "", fmt.Errorf("wrong target type in stmt: %w", err)
	}
	if err := checkPrivileges(grantStmt); err != nil {
		return "", fmt.Errorf("wrong privileges in stmt: %w", err)
	}

	if err := checkGrantReference(grantStmt); err != nil {
		return "", fmt.Errorf("wrong reference in stmt: %w", err)
	}

	if err := checkRoles(grantStmt); err != nil {
		return "", fmt.Errorf("wrong roles in stmt: %w", err)
	}

	referencedTable, err := getReferencedTable(stmt)
	if err != nil {
		return "", fmt.Errorf("get referenced table: %w", err)
	}
	return referencedTable, nil
}

func validateReadQuery(node *pg_query.Node) error {
	selectStmt := node.GetSelectStmt()

	if err := checkNoForUpdateOrShare(selectStmt); err != nil {
		return fmt.Errorf("no for check: %w", err)
	}

	return nil
}

func checkNonEmptyStatement(parsed *pg_query.ParseResult) error {
	if len(parsed.Stmts) == 0 {
		return &parsing.ErrEmptyStatement{}
	}
	return nil
}

func checkSingleStatement(parsed *pg_query.ParseResult) error {
	if len(parsed.Stmts) != 1 {
		return &parsing.ErrNoSingleStatement{}
	}
	return nil
}

func checkTopLevelUpdateInsertDelete(node *pg_query.Node) error {
	if node.GetUpdateStmt() == nil &&
		node.GetInsertStmt() == nil &&
		node.GetDeleteStmt() == nil {
		return &parsing.ErrNoTopLevelUpdateInsertDelete{}
	}
	return nil
}

func checkTopLevelGrant(node *pg_query.Node) error {
	if node.GetGrantStmt() == nil {
		return &parsing.ErrNoTopLevelGrant{}
	}
	return nil
}

func checkPrivileges(node *pg_query.GrantStmt) error {
	if node == nil {
		return errEmptyNode
	}

	// ALL PRIVILEGES is not allowed
	if len(node.GetPrivileges()) == 0 {
		return &parsing.ErrAllPrivilegesNotAllowed{}
	}

	// only INSERT, UPDATE and DELETE are allowed
	for _, privilegeNode := range node.GetPrivileges() {
		privilege := privilegeNode.GetAccessPriv().GetPrivName()
		if privilege != "insert" && privilege != "update" && privilege != "delete" {
			return &parsing.ErrNoInsertUpdateDeletePrivilege{}
		}
	}

	return nil
}

func checkGrantType(node *pg_query.GrantStmt) error {
	if node == nil {
		return errEmptyNode
	}

	if node.GetTargtype().String() != "ACL_TARGET_OBJECT" {
		return &parsing.ErrTargetTypeIsNotObject{}
	}

	return nil
}

func checkGrantReference(node *pg_query.GrantStmt) error {
	if node == nil {
		return errEmptyNode
	}

	objects := node.GetObjects()
	if len(objects) != 1 {
		return &parsing.ErrNoSingleTableReference{}
	}

	if node.GetObjtype().String() != "OBJECT_TABLE" {
		return &parsing.ErrObjectTypeIsNotTable{}
	}

	if rangeVar := objects[0].GetRangeVar(); rangeVar == nil {
		return &parsing.ErrRangeVarIsNil{}
	}

	return nil
}

func checkRoles(node *pg_query.GrantStmt) error {
	if node == nil {
		return errEmptyNode
	}

	for _, grantee := range node.GetGrantees() {
		if grantee.GetRoleSpec().Roletype.String() != "ROLESPEC_CSTRING" {
			return &parsing.ErrRoleIsNotCString{}
		}

		addr := common.Address{}
		if err := addr.UnmarshalText([]byte(grantee.GetRoleSpec().Rolename)); err != nil {
			return &parsing.ErrRoleIsNotAnEthAddress{}
		}
	}

	return nil
}

func checkTopLevelCreate(node *pg_query.Node) error {
	if node.GetCreateStmt() == nil {
		return &parsing.ErrNoTopLevelCreate{}
	}
	return nil
}

func checkNoForUpdateOrShare(node *pg_query.SelectStmt) error {
	if node == nil {
		return errEmptyNode
	}

	if len(node.LockingClause) > 0 {
		return &parsing.ErrNoForUpdateOrShare{}
	}
	return nil
}

func checkNoReturningClause(node *pg_query.Node) error {
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

func checkNoSystemTablesReferencing(node *pg_query.Node, systemTablePrefixes []string) error {
	if node == nil {
		return nil
	}
	if rangeVar := node.GetRangeVar(); rangeVar != nil {
		if hasPrefix(rangeVar.Relname, systemTablePrefixes) {
			return &parsing.ErrSystemTableReferencing{}
		}
	} else if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if hasPrefix(insertStmt.Relation.Relname, systemTablePrefixes) {
			return &parsing.ErrSystemTableReferencing{}
		}
		return checkNoSystemTablesReferencing(insertStmt.SelectStmt, systemTablePrefixes)
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		for _, fcn := range selectStmt.FromClause {
			if err := checkNoSystemTablesReferencing(fcn, systemTablePrefixes); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if hasPrefix(updateStmt.Relation.Relname, systemTablePrefixes) {
			return &parsing.ErrSystemTableReferencing{}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := checkNoSystemTablesReferencing(fcn, systemTablePrefixes); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if hasPrefix(deleteStmt.Relation.Relname, systemTablePrefixes) {
			return &parsing.ErrSystemTableReferencing{}
		}
		if err := checkNoSystemTablesReferencing(deleteStmt.WhereClause, systemTablePrefixes); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := checkNoSystemTablesReferencing(rangeSubselectStmt.Subquery, systemTablePrefixes); err != nil {
			return fmt.Errorf("subquery: %w", err)
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := checkNoSystemTablesReferencing(joinExpr.Larg, systemTablePrefixes); err != nil {
			return fmt.Errorf("join left arg: %w", err)
		}
		if err := checkNoSystemTablesReferencing(joinExpr.Rarg, systemTablePrefixes); err != nil {
			return fmt.Errorf("join right arg: %w", err)
		}
	}
	return nil
}

func checkNoRelationAlias(node *pg_query.Node) error {
	if node == nil {
		return errEmptyNode
	}

	if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if updateStmt.GetRelation().GetAlias() != nil {
			return &parsing.ErrRelationAlias{}
		}
	} else if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if insertStmt.GetRelation().GetAlias() != nil {
			return &parsing.ErrRelationAlias{}
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if deleteStmt.GetRelation().GetAlias() != nil {
			return &parsing.ErrRelationAlias{}
		}
	} else {
		return errUnexpectedNodeType
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
	} else if grantStmt := node.GetGrantStmt(); grantStmt != nil {
		// It is safe to assume Objects has always one element.
		// This is validated in the checkPrivileges call.

		return grantStmt.GetObjects()[0].GetRangeVar().GetRelname(), nil
	}
	return "", fmt.Errorf("the statement isn't an insert/update/delete")
}

func checkMaxTextValueLength(node *pg_query.Node, maxLength int) error {
	if maxLength == 0 {
		return nil
	}
	if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		if selStmt := insertStmt.SelectStmt.GetSelectStmt(); selStmt != nil {
			for _, vl := range selStmt.ValuesLists {
				if list := vl.GetList(); list != nil {
					for _, item := range list.Items {
						if err := validateAConstStringLength(item, maxLength); err != nil {
							return err
						}
					}
				}
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		for _, target := range updateStmt.TargetList {
			if resTarget := target.GetResTarget(); resTarget != nil {
				if err := validateAConstStringLength(resTarget.Val, maxLength); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func validateAConstStringLength(n *pg_query.Node, maxLength int) error {
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

// checkNonDeterministicFunctions walks the query tree and disallow references to
// functions that aren't deterministic.
func checkNonDeterministicFunctions(node *pg_query.Node) error {
	if node == nil {
		return nil
	}
	if sqlValFunc := node.GetSqlvalueFunction(); sqlValFunc != nil {
		return &parsing.ErrNonDeterministicFunction{}
	} else if listStmt := node.GetList(); listStmt != nil {
		for _, item := range listStmt.Items {
			if err := checkNonDeterministicFunctions(item); err != nil {
				return fmt.Errorf("list item: %w", err)
			}
		}
	}
	if insertStmt := node.GetInsertStmt(); insertStmt != nil {
		return checkNonDeterministicFunctions(insertStmt.SelectStmt)
	} else if selectStmt := node.GetSelectStmt(); selectStmt != nil {
		for _, nl := range selectStmt.ValuesLists {
			if err := checkNonDeterministicFunctions(nl); err != nil {
				return fmt.Errorf("value list: %w", err)
			}
		}
		for _, fcn := range selectStmt.FromClause {
			if err := checkNonDeterministicFunctions(fcn); err != nil {
				return fmt.Errorf("from: %w", err)
			}
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		for _, t := range updateStmt.TargetList {
			if err := checkNonDeterministicFunctions(t); err != nil {
				return fmt.Errorf("target: %w", err)
			}
		}
		for _, fcn := range updateStmt.FromClause {
			if err := checkNonDeterministicFunctions(fcn); err != nil {
				return fmt.Errorf("from clause: %w", err)
			}
		}
		if err := checkNonDeterministicFunctions(updateStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if err := checkNonDeterministicFunctions(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if rangeSubselectStmt := node.GetRangeSubselect(); rangeSubselectStmt != nil {
		if err := checkNonDeterministicFunctions(rangeSubselectStmt.Subquery); err != nil {
			return fmt.Errorf("subquery: %w", err)
		}
	} else if joinExpr := node.GetJoinExpr(); joinExpr != nil {
		if err := checkNonDeterministicFunctions(joinExpr.Larg); err != nil {
			return fmt.Errorf("join left tree: %w", err)
		}
		if err := checkNonDeterministicFunctions(joinExpr.Rarg); err != nil {
			return fmt.Errorf("join right tree: %w", err)
		}
	} else if aExpr := node.GetAExpr(); aExpr != nil {
		if err := checkNonDeterministicFunctions(aExpr.Lexpr); err != nil {
			return fmt.Errorf("aexpr left: %w", err)
		}
		if err := checkNonDeterministicFunctions(aExpr.Rexpr); err != nil {
			return fmt.Errorf("aexpr right: %w", err)
		}
	} else if resTarget := node.GetResTarget(); resTarget != nil {
		if err := checkNonDeterministicFunctions(resTarget.Val); err != nil {
			return fmt.Errorf("target: %w", err)
		}
	}
	return nil
}

func checkNoJoinOrSubquery(node *pg_query.Node) error {
	if node == nil {
		return nil
	}

	if resTarget := node.GetResTarget(); resTarget != nil {
		if err := checkNoJoinOrSubquery(resTarget.Val); err != nil {
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
		if err := checkNoJoinOrSubquery(insertStmt.SelectStmt); err != nil {
			return fmt.Errorf("insert select expr: %w", err)
		}
	} else if updateStmt := node.GetUpdateStmt(); updateStmt != nil {
		if len(updateStmt.FromClause) != 0 {
			return &parsing.ErrJoinOrSubquery{}
		}
		if err := checkNoJoinOrSubquery(updateStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if deleteStmt := node.GetDeleteStmt(); deleteStmt != nil {
		if err := checkNoJoinOrSubquery(deleteStmt.WhereClause); err != nil {
			return fmt.Errorf("where clause: %w", err)
		}
	} else if aExpr := node.GetAExpr(); aExpr != nil {
		if err := checkNoJoinOrSubquery(aExpr.Lexpr); err != nil {
			return fmt.Errorf("aexpr left: %w", err)
		}
		if err := checkNoJoinOrSubquery(aExpr.Rexpr); err != nil {
			return fmt.Errorf("aexpr right: %w", err)
		}
	} else if subLinkExpr := node.GetSubLink(); subLinkExpr != nil {
		return &parsing.ErrJoinOrSubquery{}
	} else if boolExpr := node.GetBoolExpr(); boolExpr != nil {
		for _, arg := range boolExpr.Args {
			if err := checkNoJoinOrSubquery(arg); err != nil {
				return fmt.Errorf("bool expr: %w", err)
			}
		}
	}
	return nil
}

func isGrant(node *pg_query.Node) bool {
	return node.GetGrantStmt() != nil
}

func isWrite(node *pg_query.Node) bool {
	return node.GetUpdateStmt() != nil || node.GetInsertStmt() != nil || node.GetDeleteStmt() != nil
}

type colNameType struct {
	colName  string
	typeName string
}

func checkCreateColTypes(createStmt *pg_query.CreateStmt, acceptedTypesNames []string) ([]colNameType, error) {
	if createStmt == nil {
		return nil, errEmptyNode
	}

	if createStmt.OfTypename != nil {
		// This will only ever be one, otherwise its a parsing error
		for _, nameNode := range createStmt.OfTypename.Names {
			if name := nameNode.GetString_(); name == nil {
				return nil, fmt.Errorf("unexpected type name node: %v", name)
			}
		}
	}

	var colNameTypes []colNameType
	for _, col := range createStmt.TableElts {
		if colConst := col.GetConstraint(); colConst != nil {
			continue
		}
		colDef := col.GetColumnDef()
		if colDef == nil {
			return nil, errors.New("unexpected node type in column definition")
		}

		var typeName string
	AcceptedTypesFor:
		for _, nameNode := range colDef.TypeName.Names {
			name := nameNode.GetString_()
			if name == nil {
				return nil, fmt.Errorf("unexpected type name node: %v", name)
			}
			// We skip `pg_catalog` since it seems that gets included for some
			// cases of native types.
			if name.Str == "pg_catalog" {
				continue
			}

			for _, atn := range acceptedTypesNames {
				if name.Str == atn {
					typeName = atn
					// The current data type name has a match with accepted
					// types. Continue matching the rest of columns.
					break AcceptedTypesFor
				}
			}

			return nil, &parsing.ErrInvalidColumnType{ColumnType: name.Str}
		}

		colNameTypes = append(colNameTypes, colNameType{colName: colDef.Colname, typeName: typeName})
	}

	return colNameTypes, nil
}

func genCreateStmt(cNode *pg_query.Node, cols []colNameType, chainID tableland.ChainID) *createStmt {
	strCols := make([]string, len(cols))
	for i := range cols {
		strCols[i] = fmt.Sprintf("%s:%s", cols[i].colName, cols[i].typeName)
	}
	stringifiedColDef := strings.Join(strCols, ",")
	sh := sha256.New()
	sh.Write([]byte(stringifiedColDef))
	hash := sh.Sum(nil)

	return &createStmt{
		chainID:       chainID,
		cNode:         cNode,
		structureHash: hex.EncodeToString(hash),
		namePrefix:    cNode.GetCreateStmt().Relation.Relname,
	}
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
	cNode         *pg_query.Node
	structureHash string
	namePrefix    string
}

var _ parsing.CreateStmt = (*createStmt)(nil)

func (cs *createStmt) GetRawQueryForTableID(id tableland.TableID) (string, error) {
	parsedTree := &pg_query.ParseResult{}

	cs.cNode.GetCreateStmt().Relation.Relname = fmt.Sprintf("_%d_%s", cs.chainID, id)
	rs := &pg_query.RawStmt{Stmt: cs.cNode}
	parsedTree.Stmts = []*pg_query.RawStmt{rs}
	wq, err := pg_query.Deparse(parsedTree)
	if err != nil {
		return "", fmt.Errorf("deparsing statement: %s", err)
	}
	return wq, nil
}
func (cs *createStmt) GetStructureHash() string {
	return cs.structureHash
}
func (cs *createStmt) GetNamePrefix() string {
	return cs.namePrefix
}
