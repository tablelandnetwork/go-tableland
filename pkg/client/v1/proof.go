package v1

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

// Proof gets a proof for a given row.
func (c *Client) Proof(
	ctx context.Context,
	tableID TableID,
	row []byte,
) ([]string, bool, error) {
	return c.getProof(ctx, tableID, row)
}

func (c *Client) getProof(ctx context.Context, tableID TableID, row []byte) ([]string, bool, error) {
	url := fmt.Sprintf(
		"%s/api/v1/proof/%d/%d/%s", c.baseURL, c.chain.ID, tableID.ToBigInt().Int64(), hex.EncodeToString(row),
	)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, false, fmt.Errorf("creating request: %s", err)
	}
	response, err := c.tblHTTP.Do(req)
	if err != nil {
		return nil, false, fmt.Errorf("calling get receipt by transaction hash: %s", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode == http.StatusNotFound {
		return nil, false, nil
	}
	if response.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(response.Body)
		return nil, false, fmt.Errorf("failed call (status: %d, body: %s)", response.StatusCode, msg)
	}

	var proof apiv1.Proof
	if err := json.NewDecoder(response.Body).Decode(&proof); err != nil {
		return nil, false, fmt.Errorf("unmarshaling result: %s", err)
	}
	return proof.Proof, true, nil
}
