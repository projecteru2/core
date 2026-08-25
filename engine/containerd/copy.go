package containerd

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	tarFatalCode = 2
	noSuchFile   = "No such file"
)

func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return e.VirtualizationCopyChunkTo(ctx, ID, target, int64(len(content)), bytes.NewReader(content), uid, gid, mode)
}

func (e *Engine) VirtualizationCopyChunkTo(ctx context.Context, ID, target string, size int64, content io.Reader, uid, gid int, mode int64) error {
	argv, err := e.tarArgv(ctx, ID, "-x", "-C", filepath.Dir(target))
	if err != nil {
		return err
	}
	reader, writer := io.Pipe()
	go func() {
		_ = writer.CloseWithError(writeTar(writer, filepath.Base(target), size, content, uid, gid, mode))
	}()
	res, err := e.runner.Run(ctx, sshrunner.Quote(argv), reader)
	_ = reader.CloseWithError(err)
	if err != nil {
		return err
	}
	return sshrunner.ExitError(argv, res)
}

func (e *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, err error) {
	argv, err := e.tarArgv(ctx, ID, "-c", "-C", filepath.Dir(path), filepath.Base(path))
	if err != nil {
		return nil, 0, 0, 0, err
	}
	res, err := e.runner.Run(ctx, sshrunner.Quote(argv), nil)
	if err != nil {
		return nil, 0, 0, 0, err
	}
	if err = sshrunner.ExitError(argv, res); err != nil {
		if missingPath(res) {
			return nil, 0, 0, 0, errors.Wrapf(coretypes.ErrWorkloadNotExists, "%s not found in workload %s", path, ID)
		}
		return nil, 0, 0, 0, err
	}
	reader := tar.NewReader(bytes.NewReader([]byte(res.Stdout)))
	header, err := reader.Next()
	if err != nil {
		return nil, 0, 0, 0, err
	}
	content, err = io.ReadAll(reader)
	return content, header.Uid, header.Gid, header.Mode, err
}

// tarArgv runs tar inside the workload; copy is a stream, and only an exec carries one.
func (e *Engine) tarArgv(ctx context.Context, ID string, args ...string) ([]string, error) {
	found, err := e.container(ctx, ID)
	if err != nil {
		return nil, err
	}
	config := &enginetypes.ExecConfig{Cmd: append([]string{"tar"}, args...)}
	return e.execArgv(found.ID(), utils.RandomID(), config), nil
}

func missingPath(res *sshrunner.Result) bool {
	return res.Code == tarFatalCode || strings.Contains(res.Stderr, noSuchFile)
}

func writeTar(out io.Writer, name string, size int64, content io.Reader, uid, gid int, mode int64) error {
	writer := tar.NewWriter(out)
	if err := writer.WriteHeader(&tar.Header{Name: name, Size: size, Mode: mode, Uid: uid, Gid: gid}); err != nil {
		return err
	}
	if _, err := io.Copy(writer, content); err != nil {
		return err
	}
	return writer.Close()
}
