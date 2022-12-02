package client

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/textileio/go-tableland/internal/router/controllers/apiv1"
)

func (c *Client) Version(ctx context.Context) (*apiv1.VersionInfo, error) {
	res, err := c.tblHTTP.Get(c.chain.Endpoint + "/version")
	if err != nil {
		return nil, fmt.Errorf("http get error: %s", err)
	}

	var versionInfo apiv1.VersionInfo
	if err := json.NewDecoder(res.Body).Decode(&versionInfo); err != nil {
		return nil, fmt.Errorf("decoding version info: %s", err)
	}

	return &versionInfo, nil
}
