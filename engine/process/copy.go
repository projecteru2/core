package process

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/cockroachdb/errors"

	coretypes "github.com/projecteru2/core/types"
)

const (
	lowerDir  = "lower"
	upperDir  = "upper"
	mergedDir = "merged"
)

func (e *Engine) VirtualizationCopyChunkTo(ctx context.Context, ID, target string, _ int64, content io.Reader, uid, gid int, mode int64) error {
	paths, err := e.hostPaths(ctx, ID, target)
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

	path := paths[0]
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
	paths, err := e.hostPaths(ctx, ID, path)
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

	for _, host := range paths {
		info, statErr := remote.Stat(host)
		if statErr != nil {
			continue
		}
		file, openErr := remote.Open(host)
		if openErr != nil {
			return nil, 0, 0, 0, openErr
		}
		content, err = io.ReadAll(file)
		_ = file.Close()
		return content, info.UID, info.GID, int64(info.Mode), err
	}
	return nil, 0, 0, 0, errors.Wrapf(coretypes.ErrWorkloadNotExists, "%s not found in workload %s", path, ID)
}

// hostPaths maps a path inside the workload onto the node's filesystem, most specific first: writing under a mounted overlay is undefined and its upper dir alone misses the bundle.
func (e *Engine) hostPaths(ctx context.Context, ID, target string) ([]string, error) {
	record, mounted, err := e.workloadMeta(ctx, ID)
	if err != nil {
		return nil, err
	}
	if record.RootDirectory == "" {
		return []string{filepath.Join(record.WorkingDir, target)}, nil
	}
	dir := workloadDir(e.root, ID)
	if mounted {
		return []string{filepath.Join(dir, mergedDir, target)}, nil
	}
	return []string{filepath.Join(dir, upperDir, target), filepath.Join(dir, lowerDir, target)}, nil
}
