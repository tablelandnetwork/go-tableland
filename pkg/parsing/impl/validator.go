package impl

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"

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
	systemTablePrefixes []string
	acceptedTypesNames  []string
	rawTablenameRegEx   *regexp.Regexp
	maxAllowedColumns   int
	maxTextLength       int
}

var _ parsing.SQLValidator = (*QueryValidator)(nil)

// New returns a Tableland query validator.
func New(systemTablePrefixes []string, maxAllowedColumns int, maxTextLength int) *QueryValidator {
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

	return genCreateStmt(stmt, colNameTypes), nil
}

// ValidateRunSQL validates the query and returns an error if isn't allowed.
// If the query validates correctly, it returns the query type and nil.
func (pp *QueryValidator) ValidateRunSQL(query string) (parsing.SugaredReadStmt, []parsing.SugaredWriteStmt, error) {
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
		refTable, err := validateReadQuery(stmt)
		if err != nil {
			return nil, nil, fmt.Errorf("validating read-query: %w", err)
		}
		namePrefix, posTableName, err := pp.deconstructRefTable(refTable)
		if err != nil {
			return nil, nil, fmt.Errorf("deconstructing referenced table name: %w", err)
		}
		return &sugaredStmt{
			node:              stmt,
			namePrefix:        namePrefix,
			postgresTableName: posTableName,
		}, nil, nil
	}

	// It's a write-query.

	// Since we support write queries with more than one statement,
	// do the write-query validation in each of them. Also, check
	// that each statement reference always the same table.
	var targetTable string
	for i := range parsed.Stmts {
		refTable, err := pp.validateWriteQuery(parsed.Stmts[i].Stmt)
		if err != nil {
			return nil, nil, fmt.Errorf("validating write-query: %w", err)
		}

		if targetTable == "" {
			targetTable = refTable
		} else if targetTable != refTable {
			return nil, nil, &parsing.ErrMultiTableReference{Ref1: targetTable, Ref2: refTable}
		}
	}

	namePrefix, posTableName, err := pp.deconstructRefTable(targetTable)
	if err != nil {
		return nil, nil, fmt.Errorf("deconstructing referenced table name: %w", err)
	}

	ret := make([]parsing.SugaredWriteStmt, len(parsed.Stmts))
	for i := range parsed.Stmts {
		ret[i] = &sugaredStmt{
			node:              parsed.Stmts[i].Stmt,
			namePrefix:        namePrefix,
			postgresTableName: posTableName,
		}
	}

	return nil, ret, nil
}

func (pp *QueryValidator) deconstructRefTable(refTable string) (string, string, error) {
	if hasPrefix(refTable, pp.systemTablePrefixes) {
		return "", refTable, nil
	}
	if !pp.rawTablenameRegEx.MatchString(refTable) {
		return "", "", &parsing.ErrInvalidTableName{}
	}

	var namePrefix, realTableName string
	sepIdx := strings.LastIndex(refTable, "_")
	if sepIdx == -1 {
		realTableName = refTable // No name prefix case, _{ID}.
	} else {
		namePrefix = refTable[:sepIdx]    // If sepIdx==0, this is correct too.
		realTableName = refTable[sepIdx:] // {name}_{id} -> _{id}
	}

	return namePrefix, realTableName, nil
}

type sugaredStmt struct {
	node              *pg_query.Node
	namePrefix        string
	postgresTableName string
}

var _ parsing.SugaredWriteStmt = (*sugaredStmt)(nil)

func (ws *sugaredStmt) GetDesugaredQuery() (string, error) {
	parsedTree := &pg_query.ParseResult{}

	if insertStmt := ws.node.GetInsertStmt(); insertStmt != nil {
		insertStmt.Relation.Relname = ws.postgresTableName
	} else if updateStmt := ws.node.GetUpdateStmt(); updateStmt != nil {
		updateStmt.Relation.Relname = ws.postgresTableName
	} else if deleteStmt := ws.node.GetDeleteStmt(); deleteStmt != nil {
		deleteStmt.Relation.Relname = ws.postgresTableName
	} else if selectStmt := ws.node.GetSelectStmt(); selectStmt != nil {
		for i := range selectStmt.FromClause {
			rangeVar := selectStmt.FromClause[i].GetRangeVar()
			if rangeVar == nil {
				return "", fmt.Errorf("select doesn't reference a table")
			}
			rangeVar.Relname = ws.postgresTableName
		}
	}
	rs := &pg_query.RawStmt{Stmt: ws.node}
	parsedTree.Stmts = []*pg_query.RawStmt{rs}
	wq, err := pg_query.Deparse(parsedTree)
	if err != nil {
		return "", fmt.Errorf("deparsing statement: %s", err)
	}
	return wq, nil
}

func (ws *sugaredStmt) GetNamePrefix() string {
	return ws.namePrefix
}

func (ws *sugaredStmt) GetTableID() tableland.TableID {
	tid, _ := tableland.NewTableID(ws.postgresTableName[1:])
	return tid
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

func validateReadQuery(node *pg_query.Node) (string, error) {
	selectStmt := node.GetSelectStmt()

	if err := checkNoJoinOrSubquery(selectStmt.WhereClause); err != nil {
		return "", fmt.Errorf("join or subquery in where: %w", err)
	}
	for _, n := range selectStmt.TargetList {
		if err := checkNoJoinOrSubquery(n); err != nil {
			return "", fmt.Errorf("join or subquery in cols: %w", err)
		}
	}

	var targetTable string
	for _, n := range selectStmt.FromClause {
		rangeVar := n.GetRangeVar()
		if rangeVar == nil {
			return "", fmt.Errorf("from clause doesn't reference a table: %w", &parsing.ErrJoinOrSubquery{})
		}

		if targetTable == "" {
			targetTable = rangeVar.Relname
			continue
		}
		// Second, and further FROMs should always
		// reference the same detected table name.
		if targetTable != rangeVar.Relname {
			return "", &parsing.ErrMultiTableReference{}
		}
	}

	if err := checkNoForUpdateOrShare(selectStmt); err != nil {
		return "", fmt.Errorf("no for check: %w", err)
	}

	return targetTable, nil
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

func genCreateStmt(cNode *pg_query.Node, cols []colNameType) *createStmt {
	strCols := make([]string, len(cols))
	for i := range cols {
		strCols[i] = fmt.Sprintf("%s:%s", cols[i].colName, cols[i].typeName)
	}
	stringifiedColDef := strings.Join(strCols, ",")
	sh := sha256.New()
	sh.Write([]byte(stringifiedColDef))
	hash := sh.Sum(nil)

	return &createStmt{
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
	cNode         *pg_query.Node
	structureHash string
	namePrefix    string
}

var _ parsing.CreateStmt = (*createStmt)(nil)

func (cs *createStmt) GetRawQueryForTableID(id tableland.TableID) (string, error) {
	parsedTree := &pg_query.ParseResult{}

	cs.cNode.GetCreateStmt().Relation.Relname = fmt.Sprintf("_%s", id)
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
