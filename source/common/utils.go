package common

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"
)

func unzipFile(body io.Reader, path string) error {
	content, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
	}

	root := filepath.Clean(path)
	for _, f := range reader.File {
		if err := extractZipEntry(f, root); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, root string) error {
	target := filepath.Join(root, f.Name) //nolint:gosec // G305: the escape check below rejects a traversing name
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		return errors.Newf("illegal path in archive: %q", f.Name)
	}

	if f.FileInfo().IsDir() {
		return os.MkdirAll(target, f.Mode())
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
		return err
	}

	zipped, err := f.Open()
	if err != nil {
		return err
	}
	defer func() {
		_ = zipped.Close()
	}()

	writer, err := os.OpenFile(filepath.Clean(target), os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
	if err != nil {
		return err
	}
	defer func() {
		_ = writer.Close()
	}()

	_, err = io.Copy(writer, zipped) //nolint:gosec // G110: extraction is not size-capped; see the report
	return err
}
