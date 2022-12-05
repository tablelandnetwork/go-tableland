package client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

var ErrTableNotFound = errors.New("table not found")

func (c *Client) GetTable(ctx context.Context, tableID TableID) (*apiv1.Table, error) {
	url := *c.baseURL.
		JoinPath("api/v1/tables/").
		JoinPath(strconv.FormatInt(int64(c.chain.ID), 10)).
		JoinPath(strconv.FormatInt(tableID.ToBigInt().Int64(), 10))

	response, err := c.tblHTTP.Get(url.String())
	if err != nil {
		return nil, fmt.Errorf("calling get tables by id: %s", err)
	}
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
