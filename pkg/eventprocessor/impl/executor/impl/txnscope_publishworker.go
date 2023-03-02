package impl

import (
	"context"
	"fmt"
	"net/http"

	"github.com/textileio/go-tableland/pkg/tables"
	"github.com/textileio/go-tableland/pkg/tables/impl/ethereum"
)

func (ts *txnScope) executePublishFunctionEvent(
	_ context.Context,
	e *ethereum.ContractPublishFunction,
) (eventExecutionResult, error) {
	tableID, err := tables.NewTableIDFromInt64(0)
	if err != nil {
		return eventExecutionResult{}, fmt.Errorf("creating fake table id: %s", err)
	}

	requestURL := fmt.Sprintf("http://localhost:%d/v1/add/%s", 3030, e.Cid)
	_, err = http.Post(requestURL, "text/plain", nil)
	if err != nil {
		return eventExecutionResult{}, fmt.Errorf("adding function: %s", err)
	}

	return eventExecutionResult{TableID: &tableID}, nil
}
