package common

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
)

// unzipFile unzip a file(from resp.Body) to the spec path
func unzipFile(body io.Reader, path string) error {
	content, err := io.ReadAll(body)
	if err != nil {
		return err
	}

	reader, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return err
	}

	// extract files from zipfile
	for _, f := range reader.File {
		zipped, err := f.Open()
		if err != nil {
			return err
		}

		defer zipped.Close()

		//  G305: File traversal when extracting zip archive
		p := filepath.Join(path, f.Name) // nolint

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(p, f.Mode())
			continue
		}

		writer, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE, f.Mode())
		if err != nil {
			return err
		}

		defer writer.Close()
		if _, err = io.Copy(writer, zipped); err != nil { // nolint
			// G110: Potential DoS vulnerability via decompression bomb
			return err
		}
	}
	return nil
}
