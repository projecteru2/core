package process

import (
	"context"
	"io"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/cockroachdb/errors"

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

// sessionReader closes the ssh session backing a follow stream.
type sessionReader struct {
	io.Reader
	sess session
}

func (r *sessionReader) Close() error {
	return r.sess.Close()
}

func (e *Engine) VirtualizationLogs(ctx context.Context, opts *enginetypes.VirtualizationLogStreamOptions) (stdout, stderr io.ReadCloser, err error) {
	unit := unitName(opts.ID)
	flags, err := journalFlags(opts)
	if err != nil {
		return nil, nil, err
	}
	if !opts.Follow {
		res, runErr := e.run(ctx, slices.Concat([]string{"journalctl", "-u", unit}, flags)...)
		if runErr != nil {
			return nil, nil, runErr
		}
		return io.NopCloser(strings.NewReader(res.Stdout)), nil, nil
	}

	running, err := e.runner.Start(ctx, quote(shell(followScript, slices.Concat([]string{unit, "-f"}, flags)...)), &startOptions{})
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

func journalFlags(opts *enginetypes.VirtualizationLogStreamOptions) ([]string, error) {
	flags := []string{"-o", "cat"}
	if opts.Tail != "" {
		flags = append(flags, "-n", opts.Tail)
	}
	if opts.Since != "" {
		stamp, err := journalTime(opts.Since)
		if err != nil {
			return nil, err
		}
		flags = append(flags, "--since", stamp)
	}
	if opts.Until != "" {
		stamp, err := journalTime(opts.Until)
		if err != nil {
			return nil, err
		}
		flags = append(flags, "--until", stamp)
	}
	return flags, nil
}

// journalTime renders core's RFC3339 or unix-seconds timestamp the way journalctl reads one.
func journalTime(value string) (string, error) {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return "@" + strconv.FormatInt(seconds, 10), nil
	}
	stamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", errors.Wrapf(coretypes.ErrInvaildWorkloadOps, "unsupported log timestamp %q", value)
	}
	return "@" + strconv.FormatInt(stamp.Unix(), 10), nil
}
