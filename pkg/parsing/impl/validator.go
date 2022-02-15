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
	"github.com/textileio/go-tableland/pkg/parsing/impl/node"
)

// var (
// 	errEmptyNode          = errors.New("empty node")
// 	errUnexpectedNodeType = errors.New("unexpected node type")
// )

// QueryValidator enforces PostgresSQL constraints for Tableland.
type QueryValidator struct {
	systemTablePrefix  string
	acceptedTypesNames []string
	rawTablenameRegEx  *regexp.Regexp
	maxAllowedColumns  int
	maxTextLength      int
}

var _ parsing.SQLValidator = (*QueryValidator)(nil)

// New returns a Tableland query validator.
func New(systemTablePrefix string, maxAllowedColumns int, maxTextLength int) *QueryValidator {
	// We create here a flattened slice of all the accepted type names from
	// the parsing.AcceptedTypes source of truth. We do this since having a
	// slice is easier and faster to do checks.
	var acceptedTypesNames []string
	for _, at := range parsing.AcceptedTypes {
		acceptedTypesNames = append(acceptedTypesNames, at.Names...)
	}

	rawTablenameRegEx, _ := regexp.Compile(`^\w*_[0-9]+$`)

	return &QueryValidator{
		systemTablePrefix:  systemTablePrefix,
		acceptedTypesNames: acceptedTypesNames,
		rawTablenameRegEx:  rawTablenameRegEx,
		maxAllowedColumns:  maxAllowedColumns,
		maxTextLength:      maxTextLength,
	}
}

// ValidateCreateTable validates the provided query and returns an error
// if the CREATE statement isn't allowed. Returns nil otherwise.
func (pp *QueryValidator) ValidateCreateTable(query string) (parsing.CreateStmt, error) {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := node.CheckNonEmptyStatement(parsed); err != nil {
		return nil, fmt.Errorf("empty-statement check: %w", err)
	}

	if err := node.CheckSingleStatement(parsed); err != nil {
		return nil, fmt.Errorf("single-statement check: %w", err)
	}

	stmt := parsed.Stmts[0].Stmt
	if err := node.CheckTopLevelCreate(stmt); err != nil {
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
func (pp *QueryValidator) ValidateRunSQL(query string) (parsing.SugaredReadStmt, []parsing.SugaredWriteStmt, []parsing.SugaredGrantStmt, error) {
	parsed, err := pg_query.Parse(query)
	if err != nil {
		return nil, nil, nil, &parsing.ErrInvalidSyntax{InternalError: err}
	}

	if err := node.CheckNonEmptyStatement(parsed); err != nil {
		return nil, nil, nil, fmt.Errorf("empty-statement check: %w", err)
	}

	stmt := parsed.Stmts[0].Stmt

	if selectStmt := stmt.GetSelectStmt(); selectStmt != nil {
		if err := node.CheckSingleStatement(parsed); err != nil {
			return nil, nil, nil, fmt.Errorf("single-statement check: %w", err)
		}
		refTable, err := validateReadQuery(stmt)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("validating read-query: %w", err)
		}
		namePrefix, posTableName, err := pp.deconstructRefTable(refTable)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("deconstructing referenced table name: %w", err)
		}
		return &sugaredStmt{
			node:              stmt,
			namePrefix:        namePrefix,
			postgresTableName: posTableName,
		}, nil, nil, nil
	}

	// It's a write-query or a grant-query.

	// Since we support write queries with more than one statement,
	// do the write-query validation in each of them. Also, check
	// that each statement reference always the same table.

	type processedStmt struct {
		n       node.TopLevelRunSQLNode
		rawNode *pg_query.Node
	}

	processedStmts := make([]processedStmt, len(parsed.Stmts))
	var targetTable string
	for i := range parsed.Stmts {
		stmt := parsed.Stmts[i].Stmt
		n, err := node.NewTopLevelRunSQLNode(stmt)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("creating node: %w", err)
		}

		if err := n.CheckRules(pp.systemTablePrefix, pp.maxTextLength); err != nil {
			return nil, nil, nil, fmt.Errorf("statement is not allowed: %w", err)
		}

		refTable, err := n.GetReferencedTable()
		if err != nil {
			return nil, nil, nil, fmt.Errorf("getting referenced table: %w", err)
		}

		if targetTable == "" {
			targetTable = refTable
		} else if targetTable != refTable {
			return nil, nil, nil, &parsing.ErrMultiTableReference{Ref1: targetTable, Ref2: refTable}
		}

		processedStmts[i] = processedStmt{n: n, rawNode: stmt}
	}

	namePrefix, posTableName, err := pp.deconstructRefTable(targetTable)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("deconstructing referenced table name: %w", err)
	}

	wss := make([]parsing.SugaredWriteStmt, len(parsed.Stmts))
	gss := make([]parsing.SugaredGrantStmt, len(parsed.Stmts))
	for i, pStmt := range processedStmts {
		if pStmt.n.IsWrite() {
			wss[i] = &sugaredStmt{
				node:              pStmt.rawNode,
				namePrefix:        namePrefix,
				postgresTableName: posTableName,
			}
		}

		if pStmt.n.IsGrant() {
			fmt.Println(namePrefix, posTableName)
			gss[i] = &sugaredStmt{
				node:              pStmt.rawNode,
				namePrefix:        namePrefix,
				postgresTableName: posTableName,
			}
		}
	}

	return nil, wss, gss, nil
}

func (pp *QueryValidator) deconstructRefTable(refTable string) (string, string, error) {
	if strings.HasPrefix(refTable, pp.systemTablePrefix) {
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

func validateReadQuery(n *pg_query.Node) (string, error) {
	selectStmt := n.GetSelectStmt()

	if err := node.CheckNoJoinOrSubquery(selectStmt.WhereClause); err != nil {
		return "", fmt.Errorf("join or subquery in where: %w", err)
	}
	for _, n := range selectStmt.TargetList {
		if err := node.CheckNoJoinOrSubquery(n); err != nil {
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

	if err := node.CheckNoForUpdateOrShare(selectStmt); err != nil {
		return "", fmt.Errorf("no for check: %w", err)
	}

	return targetTable, nil
}

type colNameType struct {
	colName  string
	typeName string
}

func checkCreateColTypes(createStmt *pg_query.CreateStmt, acceptedTypesNames []string) ([]colNameType, error) {
	if createStmt == nil {
		return nil, node.ErrEmptyNode
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
