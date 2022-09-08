package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/textileio/go-tableland/internal/tableland"
)

// Output is used to control the output format of a query specified with the "output" query param.
type Output string

const (
	// Table returns the query results as a JSON object with columns and rows properties.
	Table Output = "table"
	// Objects returns the query results as a JSON array of JSON objects. This is the default.
	Objects Output = "objects"
)

var outputsMap = map[string]Output{
	"table":   Table,
	"objects": Objects,
}

// OutputFromString converts a string into an Output.
func OutputFromString(o string) (Output, bool) {
	output, ok := outputsMap[o]
	return output, ok
}

// FormatConfig is the format configuration used.
type FormatConfig struct {
	Output  Output
	Unwrap  bool
	Extract bool
}

// FormatOption controls the behavior of calls to Format.
type FormatOption func(*FormatConfig)

// WithOutput specifies the output format. Default is Table.
func WithOutput(output Output) FormatOption {
	return func(fc *FormatConfig) {
		fc.Output = output
	}
}

// WithUnwrap specifies whether or not to unwrap the returned JSON objects from their surrounding array.
// Default is false.
func WithUnwrap(unwrap bool) FormatOption {
	return func(fc *FormatConfig) {
		fc.Unwrap = unwrap
	}
}

// WithExtract specifies whether or not to extract the JSON object
// from the single property of the surrounding JSON object.
// Default is false.
func WithExtract(extract bool) FormatOption {
	return func(fc *FormatConfig) {
		fc.Extract = extract
	}
}

// Format transforms the user rows according to the provided configuration, retuning raw json or jsonl bytes.
func Format(userRows *tableland.UserRows, opts ...FormatOption) ([]byte, FormatConfig, error) {
	c := FormatConfig{
		Output: Objects,
	}
	for _, opt := range opts {
		opt(&c)
	}

	if c.Output == Table {
		b, err := json.Marshal(userRows)
		if err != nil {
			return nil, FormatConfig{}, fmt.Errorf("marshaling to json: %v", err)
		}
		return b, c, nil
	}

	objects := toObjects(userRows)
	var err error

	if c.Extract {
		objects, err = extract(objects)
		if err != nil {
			return nil, FormatConfig{}, fmt.Errorf("extracting values: %s", err)
		}
	}

	if !c.Unwrap {
		b, err := json.Marshal(objects)
		if err != nil {
			return nil, FormatConfig{}, fmt.Errorf("marshaling to json: %v", err)
		}
		return b, c, nil
	}

	unwrapped, err := unwrap(objects)
	if err != nil {
		return nil, FormatConfig{}, fmt.Errorf("unwrapping values: %s", err)
	}
	return unwrapped, c, nil
}

func toObjects(in *tableland.UserRows) []interface{} {
	objects := make([]interface{}, len(in.Rows))
	in.Rows[0][0].Value()
	for i, row := range in.Rows {
		object := make(map[string]interface{}, len(row))
		for j, val := range row {
			object[in.Columns[j].Name] = val
		}
		objects[i] = object
	}
	return objects
}

func extract(in []interface{}) ([]interface{}, error) {
	extracted := make([]interface{}, len(in))
	for i, item := range in {
		object := item.(map[string]interface{})
		if len(object) != 1 {
			return nil, fmt.Errorf("can only extract values for result sets with one column but this has %d", len(object))
		}
		for _, val := range object {
			extracted[i] = val
			break
		}
	}
	return extracted, nil
}

func unwrap(in []interface{}) ([]byte, error) {
	buf := bytes.NewBuffer([]byte{})
	for i, item := range in {
		if i != 0 {
			_, _ = buf.Write([]byte("\n"))
		}
		b, err := json.Marshal(item)
		if err != nil {
			return nil, err
		}
		_, _ = buf.Write(b)
	}
	return buf.Bytes(), nil
}
