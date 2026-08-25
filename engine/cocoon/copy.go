package cocoon

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) VirtualizationCopyTo(ctx context.Context, ID, target string, content []byte, uid, gid int, mode int64) error {
	return e.VirtualizationCopyChunkTo(ctx, ID, target, int64(len(content)), bytes.NewReader(content), uid, gid, mode)
}

// VirtualizationCopyChunkTo streams one tar entry into `tar -x -P`; the absolute name makes tar create the parents.
func (e *Engine) VirtualizationCopyChunkTo(ctx context.Context, ID, target string, size int64, content io.Reader, uid, gid int, mode int64) error {
	argv := e.vm("exec", "-i", ID, "--", "tar", "-x", "-P", "-f", "-")
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{Stdin: true})
	if err != nil {
		return err
	}
	defer func() {
		_ = running.Close()
	}()

	archive := tar.NewWriter(running.Stdin())
	header := &tar.Header{Name: target, Size: size, Mode: mode, Uid: uid, Gid: gid, ModTime: time.Now()}
	if err = archive.WriteHeader(header); err != nil {
		return err
	}
	if _, err = io.Copy(archive, content); err != nil {
		return err
	}
	if err = archive.Close(); err != nil {
		return err
	}
	if err = running.Stdin().Close(); err != nil {
		return err
	}
	return exited(argv, running)
}

func (e *Engine) VirtualizationCopyFrom(ctx context.Context, ID, path string) (content []byte, uid, gid int, mode int64, err error) {
	running, err := e.runner.Start(ctx, sshrunner.Quote(e.vm("exec", ID, "--", "tar", "-c", "-P", "-f", "-", path)), &sshrunner.StartOptions{})
	if err != nil {
		return nil, 0, 0, 0, err
	}
	defer func() {
		_ = running.Close()
	}()

	archive := tar.NewReader(running.Stdout())
	header, err := archive.Next()
	if err != nil {
		return nil, 0, 0, 0, errors.Wrapf(coretypes.ErrWorkloadNotExists, "%s not found in workload %s", path, ID)
	}
	content, err = io.ReadAll(archive)
	return content, header.Uid, header.Gid, header.Mode, err
}

// exited reports a non-zero guest exit with what the command said on stderr.
func exited(argv []string, running sshrunner.Session) error {
	stderr, _ := io.ReadAll(running.Stderr())
	code, err := running.Wait()
	if err != nil {
		return err
	}
	return sshrunner.ExitError(argv, &sshrunner.Result{Stderr: strings.TrimSpace(string(stderr)), Code: code})
}
