package docker

import (
	"archive/tar"
	"context"
	"os"
	"path/filepath"

	"github.com/projecteru2/core/log"
)

func withTarfileDump(ctx context.Context, target string, content []byte, uid, gid int, mode int64, f func(target, tarfile string) error) error {
	tarfile, err := tempTarFile(target, content, uid, gid, mode)

	defer func(tarfile string) {
		if err := os.RemoveAll(tarfile); err != nil {
			log.WithFunc("engine.docker.withTarfileDump").Warnf(ctx, "clean dump files failed: %+v", err)
		}
	}(tarfile)

	if err != nil {
		return err
	}
	return f(target, tarfile)
}

func tempTarFile(path string, data []byte, uid, gid int, mode int64) (string, error) {
	filename := filepath.Base(path)
	f, err := os.CreateTemp(os.TempDir(), filename)
	if err != nil {
		return "", err
	}
	name := f.Name()
	defer f.Close()

	tw := tar.NewWriter(f)
	defer tw.Close()
	hdr := &tar.Header{
		Name: filename,
		Size: int64(len(data)),
		Mode: mode,
		Uid:  uid,
		Gid:  gid,
	}
	if err := tw.WriteHeader(hdr); err != nil {
		return name, err
	}
	_, err = tw.Write(data)
	return name, err
}
