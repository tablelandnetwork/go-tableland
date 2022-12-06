package client

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Output is used to control the output format of a Read using the ReadOutput option.
type Output string

const (
	// Table returns the query results as a JSON object with columns and rows properties.
	Table Output = "table"
	// Objects returns the query results as a JSON array of JSON objects. This is the default.
	Objects Output = "objects"
)

type readQueryParameters struct {
	format  Output
	extract bool
	unwrap  bool
}

var defaultReadQueryParameters = readQueryParameters{
	format:  Objects,
	extract: false,
	unwrap:  false,
}

// ReadOption controls the behavior of Read.
type ReadOption func(*readQueryParameters)

// ReadOutput sets the output format. Default is Objects.
func ReadOutput(output Output) ReadOption {
	return func(params *readQueryParameters) {
		params.format = output
	}
}

// ReadExtract specifies whether or not to extract the JSON object
// from the single property of the surrounding JSON object.
// Default is false.
func ReadExtract() ReadOption {
	return func(params *readQueryParameters) {
		params.extract = true
	}
}

// ReadUnwrap specifies whether or not to unwrap the returned JSON objects from their surrounding array.
// Default is false.
func ReadUnwrap() ReadOption {
	return func(params *readQueryParameters) {
		params.unwrap = true
	}
}

// Read runs a read query with the provided opts and unmarshals the results into target.
func (c *Client) Read(ctx context.Context, query string, target interface{}, opts ...ReadOption) error {
	params := defaultReadQueryParameters
	for _, opt := range opts {
		opt(&params)
	}

	url := *c.baseURL.JoinPath("api/v1/query")

	values := url.Query()
	values.Set("statement", query)
	values.Set("format", string(params.format))
	if params.extract {
		values.Set("extract", "true")
	}
	if params.unwrap {
		values.Set("unwrap", "true")
	}
	url.RawQuery = values.Encode()

	req, err := http.NewRequestWithContext(ctx, "GET", url.String(), nil)
	if err != nil {
		return fmt.Errorf("creating request: %s", err)
	}
	response, err := c.tblHTTP.Do(req)
	if err != nil {
		return fmt.Errorf("calling query: %s", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		msg, _ := io.ReadAll(response.Body)
		return fmt.Errorf("the response wasn't successful (status: %d, body: %s)", response.StatusCode, msg)
	}

	if err := json.NewDecoder(response.Body).Decode(&target); err != nil {
		return fmt.Errorf("decoding result into struct: %s", err)
	}

	return nil
}
