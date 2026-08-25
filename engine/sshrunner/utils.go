package sshrunner

import (
	"net"
	"slices"
	"strings"

	"github.com/cockroachdb/errors"

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
	words := make([]string, len(argv))
	for i, arg := range argv {
		words[i] = "'" + strings.ReplaceAll(arg, "'", `'\''`) + "'"
	}
	return strings.Join(words, " ")
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
