package docker

import (
	"archive/tar"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/projecteru2/core/log"
)

func withTarfileDump(target string, content io.Reader, f func(target, tarfile string) error) error {
	bytes, err := ioutil.ReadAll(content)
	if err != nil {
		return err
	}
	tarfile, err := tempTarFile(target, bytes)

	defer func(tarfile string) {
		if err := os.RemoveAll(tarfile); err != nil {
			log.Warnf("[cleanDumpFiles] clean dump files failed: %v", err)
		}
	}(tarfile)

	if err != nil {
		return err
	}
	return f(target, tarfile)
}

func tempTarFile(path string, data []byte) (string, error) {
	filename := filepath.Base(path)
	f, err := ioutil.TempFile(os.TempDir(), filename)
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()
	hdr := &tar.Header{
		Name: filename,
		Mode: 0755,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return name, err
	}
	_, err = tw.Write(data)
	return name, err
}
