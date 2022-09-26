package publisher

import (
	"context"
	"errors"
	"sync"

	"github.com/textileio/go-tableland/pkg/telemetry"
)

// MockExporter is a mocked exporter.
type MockExporter struct {
	mu      sync.Mutex
	metrics []telemetry.Metric
}

// Export stores metrcis in memory. Used in tests.
func (e *MockExporter) Export(_ context.Context, metrics []telemetry.Metric) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.metrics = append(e.metrics, metrics...)
	return nil
}

// Len returns the length of the metrics slice.
func (e *MockExporter) Len() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	return len(e.metrics)
}

// HTTPExporter exports metrics by making an HTTP request.
type HTTPExporter struct{}

// Export exports metrics by HTTP.
func (e *HTTPExporter) Export(_ context.Context, _ []telemetry.Metric) error {
	return errors.New("not implemented")
}
