package sshrunner

import (
	"context"
	"io"
	"net"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"

	coretypes "github.com/projecteru2/core/types"
)

const defaultPort = "22"

// ParseEndpoint splits <prefix>[user@]host[:port] into its ssh user, host and dial address.
func ParseEndpoint(endpoint, prefix string) (user, host, addr string, err error) {
	target, ok := strings.CutPrefix(endpoint, prefix)
	if !ok {
		return "", "", "", errors.Wrapf(coretypes.ErrInvaildEngineEndpoint, "endpoint %s", endpoint)
	}
	if name, rest, found := strings.Cut(target, "@"); found {
		user, target = name, rest
	}
	host, port, err := net.SplitHostPort(target)
	if err != nil {
		host, port = strings.Trim(target, "[]"), defaultPort
	}
	if host == "" || port == "" {
		return "", "", "", errors.Wrapf(coretypes.ErrInvaildEngineEndpoint, "endpoint %s", endpoint)
	}
	return user, host, net.JoinHostPort(host, port), nil
}

// Quote renders argv as one shell line, so no argument is ever interpolated.
func Quote(argv []string) string {
	var b strings.Builder
	for i, arg := range argv {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteByte('\'')
		b.WriteString(strings.ReplaceAll(arg, "'", `'\''`))
		b.WriteByte('\'')
	}
	return b.String()
}

// Shell wraps a script body into an argv whose positional parameters carry args.
func Shell(body string, args ...string) []string {
	return slices.Concat([]string{"sh", "-c", body, "sh"}, args)
}

// ExitError reports a non-zero exit as an error naming the command that failed.
func ExitError(argv []string, res *Result) error {
	if res.Code == 0 {
		return nil
	}
	return errors.Newf("%s exited %d: %s", argv[0], res.Code, strings.TrimSpace(res.Stderr))
}

// Call runs argv on the node; a non-zero exit is reported in the result, not as an error.
func Call(ctx context.Context, runner Runner, argv ...string) (*Result, error) {
	return runner.Run(ctx, Quote(argv), nil)
}

// Run runs argv on the node and reports a non-zero exit as an error.
func Run(ctx context.Context, runner Runner, argv ...string) (*Result, error) {
	res, err := Call(ctx, runner, argv...)
	if err != nil {
		return nil, err
	}
	return res, ExitError(argv, res)
}

// Stream runs argv on a stream connection and reports a non-zero exit as an error, for commands that outlive a control call.
func Stream(ctx context.Context, runner Runner, argv ...string) (*Result, error) {
	running, err := runner.Start(ctx, Quote(argv), &StartOptions{})
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = running.Close()
	}()
	return Exited(argv, running)
}

// Exited drains a started session and reports a non-zero exit as an error.
func Exited(argv []string, running Session) (*Result, error) {
	var out, errOut []byte
	var readers errgroup.Group
	readers.Go(func() (err error) { out, err = io.ReadAll(running.Stdout()); return err })
	readers.Go(func() (err error) { errOut, err = io.ReadAll(running.Stderr()); return err })
	if err := readers.Wait(); err != nil {
		return nil, err
	}
	code, err := running.Wait()
	if err != nil {
		return nil, err
	}
	res := &Result{Stdout: string(out), Stderr: string(errOut), Code: code}
	return res, ExitError(argv, res)
}
