package client

import (
	"context"
	"fmt"
	"net/http"
)

func (c *Client) CheckHealth(ctx context.Context) (bool, error) {
	url := *c.baseURL.JoinPath("api/v1/health")
	res, err := c.tblHTTP.Get(url.String())
	if err != nil {
		return false, fmt.Errorf("http get error: %s", err)
	}

	return res.StatusCode == http.StatusOK, nil
}
