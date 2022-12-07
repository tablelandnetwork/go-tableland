package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

// ErrTableNotFound is returned if the provided table ID isn't found in the network.
var ErrTableNotFound = errors.New("table not found")

// GetTable returns the table information given its ID. If the table ID doesn't exist,
// it returns ErrTableNotFound.
func (c *Client) GetTable(ctx context.Context, tableID TableID) (*apiv1.Table, error) {
	url := fmt.Sprintf("%s/api/v1/tables/%d/%d", c.baseURL, c.chain.ID, tableID.ToBigInt().Uint64())
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %s", err)
	}
	response, err := c.tblHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling get tables by id: %s", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return nil, ErrTableNotFound
	}
	if response.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(response.Body)
		return nil, fmt.Errorf("failed call (status: %d, body: %s)", response.StatusCode, msg)
	}
	var table apiv1.Table
	if err := json.NewDecoder(response.Body).Decode(&table); err != nil {
		return nil, fmt.Errorf("unmarshaling result: %s", err)
	}

	return &table, nil
}
