package client

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) CheckHealth(ctx context.Context) (bool, error) {
	res, err := c.tblHTTP.Get(c.chain.Endpoint + "/health")
	if err != nil {
		return false, fmt.Errorf("http get error: %s", err)
	}

	return res.StatusCode == http.StatusOK, nil
}
