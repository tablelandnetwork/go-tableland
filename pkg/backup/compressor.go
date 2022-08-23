package backup

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/ioutil"
)

// Compress compresses a file using gzip.
func Compress(filepath string) (string, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("read all: %s", err)
	}
	var b bytes.Buffer
	w := gzip.NewWriter(&b)
	if _, err := w.Write(data); err != nil {
		return "", fmt.Errorf("gzip write: %s", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close: %s", err)
	}

	newFilepath := fmt.Sprintf("%s.gz", filepath)
	if err := ioutil.WriteFile(newFilepath, b.Bytes(), 0o660); err != nil {
		return "", fmt.Errorf("write file: %s", err)
	}

	return newFilepath, nil
}
