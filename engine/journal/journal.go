// Package journal renders core's log stream options as journalctl arguments.
package journal

import (
	"context"
	"io"
	"strconv"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

// Identifier is the SyslogIdentifier every eru workload logs under.
const Identifier = "eru"

// Flags renders the tail, since and until window as journalctl flags.
func Flags(opts *enginetypes.VirtualizationLogStreamOptions) ([]string, error) {
	flags := []string{"-o", "cat"}
	if opts.Tail != "" {
		flags = append(flags, "-n", opts.Tail)
	}
	if opts.Since != "" {
		stamp, err := timestamp(opts.Since)
		if err != nil {
			return nil, err
		}
		flags = append(flags, "--since", stamp)
	}
	if opts.Until != "" {
		stamp, err := timestamp(opts.Until)
		if err != nil {
			return nil, err
		}
		flags = append(flags, "--until", stamp)
	}
	return flags, nil
}

func Read(ctx context.Context, runner sshrunner.Runner, argv ...string) (io.ReadCloser, error) {
	running, err := runner.Start(ctx, sshrunner.Quote(argv), &sshrunner.StartOptions{})
	if err != nil {
		return nil, err
	}
	return sshrunner.Reader(running), nil
}

// timestamp renders core's RFC3339 or unix-seconds timestamp the way journalctl reads one.
func timestamp(value string) (string, error) {
	if seconds, err := strconv.ParseInt(value, 10, 64); err == nil {
		return "@" + strconv.FormatInt(seconds, 10), nil
	}
	stamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return "", errors.Wrapf(coretypes.ErrInvaildWorkloadOps, "unsupported log timestamp %q", value)
	}
	return "@" + strconv.FormatInt(stamp.Unix(), 10), nil
}
