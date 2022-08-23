package backup

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"os"
)

// Compress compresses a file using gzip.
func Compress(filepath string) (string, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return "", fmt.Errorf("read all: %s", err)
	}
	var b bytes.Buffer
	w, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
	if err != nil {
		return "", fmt.Errorf("new writer level: %s", err)
	}
	if _, err := w.Write(data); err != nil {
		return "", fmt.Errorf("gzip write: %s", err)
	}
	if err := w.Close(); err != nil {
		return "", fmt.Errorf("close: %s", err)
	}

	newFilepath := fmt.Sprintf("%s.gz", filepath)
	if err := os.WriteFile(newFilepath, b.Bytes(), 0o755); err != nil {
		return "", fmt.Errorf("write file: %s", err)
	}

	return newFilepath, nil
}
