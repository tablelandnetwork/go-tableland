package v1

import (
	"context"
	"fmt"
	"net/http"
)

// CheckHealth returns true if the targeted validator endpoint is considered healthy, and false otherwise.
func (c *Client) CheckHealth(ctx context.Context) (bool, error) {
	url := fmt.Sprintf("%s/api/v1/health", c.baseURL)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, fmt.Errorf("creating request: %s", err)
	}
	res, err := c.tblHTTP.Do(req)
	if err != nil {
		return false, fmt.Errorf("http get error: %s", err)
	}
	defer func() { _ = res.Body.Close() }()

	return res.StatusCode == http.StatusOK, nil
}
