package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

// Version returns the validator version information.
func (c *Client) Version(ctx context.Context) (*apiv1.VersionInfo, error) {
	url := *c.baseURL.JoinPath("api/v1/version")

	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %s", err)
	}
	res, err := c.tblHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http get error: %s", err)
	}
	defer func() { _ = res.Body.Close() }()

	bb, _ := io.ReadAll(res.Body)

	var versionInfo apiv1.VersionInfo
	if err := json.NewDecoder(bytes.NewReader(bb)).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("decoding version info: %s", err)
	}

	return &versionInfo, nil
}
