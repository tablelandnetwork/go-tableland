package publisher

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/pkg/errors"
	"github.com/textileio/go-tableland/pkg/telemetry"
)

// HTTPExporter exports metrics by making an HTTP request.
type HTTPExporter struct {
	url string
}

// NewHTTPExporter creates an HTTPExporter.
func NewHTTPExporter(endpoint string) (*HTTPExporter, error) {
	if endpoint == "" {
		return nil, errors.New("empty url")
	}

	if _, err := url.ParseRequestURI(endpoint); err != nil {
		return nil, fmt.Errorf("invalid url: %s", err)
	}

	return &HTTPExporter{
		url: endpoint,
	}, nil
}

// Export exports metrics by HTTP.
func (e *HTTPExporter) Export(ctx context.Context, metrics []telemetry.Metric, nodeID string) error {
	body, _ := json.Marshal(map[string]interface{}{
		"node_id": nodeID,
		"metrics": metrics,
	})

	req, err := http.NewRequestWithContext(ctx, "POST", e.url, bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("creating request: %s", err)
	}

	client := http.DefaultClient
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("posting metrics: %s", err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return errors.Errorf("status code: %d", resp.StatusCode)
	}

	return nil
}
