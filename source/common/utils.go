package common

import (
	"archive/zip"
	"bytes"
	"io"
	"os"
	"path/filepath"
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

	for _, f := range reader.File {
		zipped, err := f.Open()
		if err != nil {
			return err
		}

		defer func() {
			_ = zipped.Close()
		}()

		//  G305: File traversal when extracting zip archive
		p := filepath.Join(path, f.Name) //nolint

		if f.FileInfo().IsDir() {
			_ = os.MkdirAll(p, f.Mode())
			continue
		}

		writer, err := os.OpenFile(filepath.Clean(p), os.O_WRONLY|os.O_CREATE, f.Mode())
		if err != nil {
			return err
		}

		defer func() {
			_ = writer.Close()
		}()
		if _, err = io.Copy(writer, zipped); err != nil { //nolint
			// G110: Potential DoS vulnerability via decompression bomb
			return err
		}
	}
	return nil
}
