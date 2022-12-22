package readqueryresolver

import (
	"github.com/textileio/go-tableland/internal/tableland"
	"github.com/textileio/go-tableland/pkg/eventprocessor"
)

type ReadQueryResolver struct {
	leb map[tableland.ChainID]func() int64
}

func New(chainStacks map[tableland.ChainID]eventprocessor.EventProcessor) *ReadQueryResolver {
	leb := make(map[tableland.ChainID]func() int64, len(chainStacks))
	for chainID, ep := range chainStacks {
		leb[chainID] = ep.GetLastExecutedBlockNumber
	}
	return &ReadQueryResolver{leb: leb}
}

func (rqr *ReadQueryResolver) GetBlockNumber(chainID tableland.ChainID) (int64, bool) {
	r, ok := rqr.leb[chainID]
	if !ok {
		return 0, false
	}

	return r(), true
}
