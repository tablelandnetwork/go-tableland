package backup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/klauspost/compress/zstd"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

const extension = "zst"

// Compress compresses a file using zstd.
func Compress(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("open file: %s", err)
	}
	pr, pw := io.Pipe()
	gzW, err := zstd.NewWriter(pw)
	if err != nil {
		return "", fmt.Errorf("new writer level: %s", err)
	}

	errs := errgroup.Group{}
	errs.Go(func() error {
		if _, err := io.Copy(gzW, file); err != nil {
			return errors.Errorf("copy to writer: %s", err)
		}

		if err := gzW.Close(); err != nil {
			return errors.Errorf("closing writer: %s", err)
		}

		if err := pw.Close(); err != nil {
			return errors.Errorf("closing pipe writer: %s", err)
		}

		return nil
	})

	newFilepath := fmt.Sprintf("%s.%s", filepath, extension)
	df, err := os.OpenFile(newFilepath, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return "", errors.Errorf("open new file: %s", err)
	}

	rr := bufio.NewReader(pr)
	if _, err := io.Copy(df, rr); err != nil {
		return "", errors.Errorf("copy dest file: %s", err)
	}

	if err := errs.Wait(); err != nil {
		return "", errors.Errorf("errgroup wait: %s", err)
	}
	return newFilepath, nil
}

// Decompress decompresses a zstd file.
func Decompress(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", fmt.Errorf("open file: %s", err)
	}
	pr, pw := io.Pipe()
	gzR, err := zstd.NewReader(file)
	if err != nil {
		return "", fmt.Errorf("new writer level: %s", err)
	}

	errs := errgroup.Group{}
	errs.Go(func() error {
		if _, err := io.Copy(pw, gzR); err != nil {
			return errors.Errorf("copy to writer: %s", err)
		}

		gzR.Close()
		if err := pw.Close(); err != nil {
			return errors.Errorf("closing pipe writer: %s", err)
		}

		return nil
	})

	newFilepath := strings.TrimSuffix(filepath, "."+extension)
	df, err := os.OpenFile(newFilepath, os.O_CREATE|os.O_WRONLY, 0o755)
	if err != nil {
		return "", errors.Errorf("open new file: %s", err)
	}

	writer := bufio.NewWriter(df)
	if _, err := io.Copy(writer, pr); err != nil {
		return "", errors.Errorf("copy dest file: %s", err)
	}

	if err := errs.Wait(); err != nil {
		return "", errors.Errorf("errgroup wait: %s", err)
	}
	return newFilepath, nil
}
