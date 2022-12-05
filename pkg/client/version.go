package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

func (c *Client) Version(ctx context.Context) (*apiv1.VersionInfo, error) {
	url := *c.baseURL.JoinPath("api/v1/version")
	res, err := c.tblHTTP.Get(url.String())
	if err != nil {
		return nil, fmt.Errorf("http get error: %s", err)
	}

	bb, _ := io.ReadAll(res.Body)

	var versionInfo apiv1.VersionInfo
	if err := json.NewDecoder(bytes.NewReader(bb)).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("decoding version info: %s", err)
	}

	return &versionInfo, nil
}
