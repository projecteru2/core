package process

import (
	"context"
	"io"
	"slices"

	"github.com/projecteru2/core/engine/journal"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

// journalctl -f never ends by itself, and RunAndWait drains the stream before it reads the exit code.
const followScript = `unit=$1; shift
journalctl -u "$unit" "$@" &
reader=$!
while [ "$(systemctl show "$unit" -p SubState --value)" = ` + subStateRunning + ` ]; do sleep 1; done
sleep 1
kill "$reader" 2>/dev/null || true
wait "$reader" 2>/dev/null || true
`

func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	unit := unitName(opts.ID)
	flags, err := journal.Flags(opts)
	if err != nil {
		return nil, nil, err
	}
	if !opts.Follow {
		stdout, runErr := journal.Read(ctx, e.runner, slices.Concat([]string{"journalctl", "-u", unit}, flags)...)
		return stdout, nil, runErr
	}

	running, err := e.runner.Start(ctx, sshrunner.Quote(sshrunner.Shell(followScript, slices.Concat([]string{unit, "-f"}, flags)...)), &sshrunner.StartOptions{})
	if err != nil {
		return nil, nil, err
	}
	return sshrunner.Reader(running), nil, nil
}

func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if stdin {
		return nil, nil, nil, coretypes.ErrEngineNotImplemented
	}
	stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
	return stdout, stderr, nil, err
}
