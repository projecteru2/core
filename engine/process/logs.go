package process

import (
	"context"
	"io"
	"strings"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	argv := journalArgv(unitName(opts.ID), opts)
	if !opts.Follow {
		res, runErr := e.run(ctx, argv...)
		if runErr != nil {
			return nil, nil, runErr
		}
		return io.NopCloser(strings.NewReader(res.Stdout)), nil, nil
	}
	running, err := e.runner.Start(ctx, quote(argv), &startOptions{})
	if err != nil {
		return nil, nil, err
	}
	return &sessionReader{Reader: running.Stdout(), sess: running}, nil, nil
}

func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if stdin {
		return nil, nil, nil, coretypes.ErrEngineNotImplemented
	}
	stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
	return stdout, stderr, nil, err
}

// sessionReader closes the ssh session backing a follow stream.
type sessionReader struct {
	io.Reader
	sess session
}

func (r *sessionReader) Close() error {
	return r.sess.Close()
}

func journalArgv(unit string, opts *enginetypes.VirtualizationLogStreamOptions) []string {
	argv := []string{"journalctl", "-u", unit, "-o", "cat"}
	if opts.Follow {
		argv = append(argv, "-f")
	}
	if opts.Tail != "" {
		argv = append(argv, "-n", opts.Tail)
	}
	if opts.Since != "" {
		argv = append(argv, "--since", opts.Since)
	}
	if opts.Until != "" {
		argv = append(argv, "--until", opts.Until)
	}
	return argv
}
