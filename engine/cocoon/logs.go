package cocoon

import (
	"context"
	"io"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/journal"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

// followScript ends the journal follow once the status stream shows the guest no longer running.
const followScript = `bin=$1; vm=$2; shift 2
journalctl "$@" &
reader=$!
"$bin" vm status --event --format json "$vm" | grep -q -v '"state":"running"'
sleep 1
kill "$reader" 2>/dev/null || true
wait "$reader" 2>/dev/null || true
`

// sessionReader closes the ssh session backing a follow stream.
type sessionReader struct {
	io.Reader
	sess sshrunner.Session
}

func (r *sessionReader) Close() error {
	return r.sess.Close()
}

// VirtualizationLogs reads what eru-agent copied from the guest console into journald.
func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	flags, err := journal.Flags(opts)
	if err != nil {
		return nil, nil, err
	}
	fields := []string{"SYSLOG_IDENTIFIER=" + journal.Identifier, "ERU_ID=" + opts.ID}
	if !opts.Follow {
		res, runErr := e.run(ctx, slices.Concat([]string{"journalctl"}, fields, flags)...)
		if runErr != nil {
			return nil, nil, runErr
		}
		return io.NopCloser(strings.NewReader(res.Stdout)), nil, nil
	}

	argv := sshrunner.Shell(followScript, slices.Concat([]string{e.cocoon.Binary, opts.ID}, fields, []string{"-f"}, flags)...)
	running, err := e.runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{})
	if err != nil {
		return nil, nil, err
	}
	return &sessionReader{Reader: running.Stdout(), sess: running}, nil, nil
}

// VirtualizationAttach without stdin is the journald follow; the console needs a pty (projecteru2/core#660).
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if stdin {
		return nil, nil, nil, errors.Wrap(coretypes.ErrEngineNotImplemented, "the guest console needs a pty (projecteru2/core#660)")
	}
	stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
	return stdout, stderr, nil, err
}
