package process

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
)

const upperDir = "upper"

func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return e.VirtualizationCopyChunkTo(ctx, ID, target, int64(len(content)), bytes.NewReader(content), uid, gid, mode)
}

func (e *Engine) VirtualizationCopyChunkTo(ctx context.Context, ID, target string, _ int64, content io.Reader, uid, gid int, mode int64) error {
	path, err := e.hostPath(ctx, ID, target)
	if err != nil {
		return err
	}
	remote, err := e.runner.Files(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = remote.Close()
	}()

	if err = remote.MkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	file, err := remote.Create(path)
	if err != nil {
		return err
	}
	if _, err = io.Copy(file, content); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	if err = remote.Chmod(path, os.FileMode(mode)); err != nil { //nolint:gosec // the mode comes from the caller's tar header
		return err
	}
	return remote.Chown(path, uid, gid)
}

func (e *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, err error) {
	host, err := e.hostPath(ctx, ID, path)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	remote, err := e.runner.Files(ctx)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	defer func() {
		_ = remote.Close()
	}()

	info, err := remote.Stat(host)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	file, err := remote.Open(host)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	defer func() {
		_ = file.Close()
	}()
	content, err = io.ReadAll(file)
	return content, info.UID, info.GID, int64(info.Mode), err
}

// hostPath maps a path inside the workload onto the node's filesystem.
func (e *Engine) hostPath(ctx context.Context, ID, target string) (string, error) {
	record, err := e.workloadMeta(ctx, ID)
	if err != nil {
		return "", err
	}
	if record.RootDirectory == "" {
		return filepath.Join(record.WorkingDir, target), nil
	}
	return filepath.Join(workloadDir(e.root, ID), upperDir, target), nil
}
