package parsing

import (
	"errors"
	"strconv"
	"strings"

	"github.com/tablelandnetwork/sqlparser"
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/sharedmemory"
)

// ReadStatementResolver implements the interface for custom functions resolution of read statements.
type ReadStatementResolver struct {
	sm     *sharedmemory.SharedMemory
	values []sqlparser.Expr
}

// NewReadStatementResolver creates a new ReadStatementResolver.
func NewReadStatementResolver(sm *sharedmemory.SharedMemory) *ReadStatementResolver {
	return &ReadStatementResolver{sm: sm, values: make([]sqlparser.Expr, 0)}
}

// GetBlockNumber returns the block number for a given chain id.
func (rqr *ReadStatementResolver) GetBlockNumber(chainID int64) (int64, bool) {
	return rqr.sm.GetLastSeenBlockNumber(tableland.ChainID(chainID))
}

// GetBindValues returns a slice of values to be bound to their respective parameters.
func (rqr *ReadStatementResolver) GetBindValues() []sqlparser.Expr {
	return rqr.values
}

// PrepareParams prepare the params to the correct type.
func (rqr *ReadStatementResolver) PrepareParams(params []string) error {
	values := make([]sqlparser.Expr, len(params))
	for i, param := range params {
		if strings.EqualFold(strings.ToLower(param), "null") {
			values[i] = &sqlparser.NullValue{}
			continue
		}

		if strings.EqualFold(strings.ToLower(param), "true") {
			values[i] = sqlparser.BoolValue(true)
			continue
		}

		if strings.EqualFold(strings.ToLower(param), "false") {
			values[i] = sqlparser.BoolValue(false)
			continue
		}

		if strings.HasPrefix(param, "'") && strings.HasSuffix(param, "'") {
			values[i] = &sqlparser.Value{Type: sqlparser.StrValue, Value: []byte(param[1 : len(param)-1])}
			continue
		}

		if strings.HasPrefix(param, "\"") && strings.HasSuffix(param, "\"") {
			values[i] = &sqlparser.Value{Type: sqlparser.StrValue, Value: []byte(param[1 : len(param)-1])}
			continue
		}

		if _, err := strconv.ParseInt(param, 10, 64); err == nil {
			values[i] = &sqlparser.Value{Type: sqlparser.IntValue, Value: []byte(param)}
			continue
		}

		return errors.New("unknown param type")
	}

	rqr.values = values

	return nil
}
