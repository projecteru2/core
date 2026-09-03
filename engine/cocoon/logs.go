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
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
mkfifo "$tmp/events"
journalctl "$@" &
reader=$!
"$bin" vm status --event --format json -n 1 "$vm" > "$tmp/events" &
watcher=$!
while IFS= read -r event; do
case "$event" in *'"state":"running"'*) ;; *) break;; esac
done < "$tmp/events"
sleep 1
kill "$reader" "$watcher" 2>/dev/null || true
wait "$reader" "$watcher" 2>/dev/null || true
`

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
	return sshrunner.Reader(running), nil, nil
}

// VirtualizationAttach without stdin is the journald follow; the console needs a pty (projecteru2/core#660).
func (e *Engine) VirtualizationAttach(ctx context.Context, ID string, _, stdin bool) (io.ReadCloser, io.ReadCloser, io.WriteCloser, error) {
	if stdin {
		return nil, nil, nil, errors.Wrap(coretypes.ErrEngineNotImplemented, "the guest console needs a pty (projecteru2/core#660)")
	}
	stdout, stderr, err := e.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{ID: ID, Follow: true})
	return stdout, stderr, nil, err
}
