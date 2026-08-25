package sshrunner

import (
	"context"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	kiB        = 1024
	infoFields = 4

	infoScript = `mkdir -p "$1" 2>/dev/null || true
printf '%s\n' "$(cat /etc/machine-id 2>/dev/null)" "$(nproc 2>/dev/null)" ` +
		`"$(awk '/^MemTotal:/{print $2}' /proc/meminfo 2>/dev/null)" "$(df -Pk "$1" 2>/dev/null | awk 'NR==2{print $2}')"`
)

func NodeInfo(ctx context.Context, runner Runner, root string) (*enginetypes.Info, error) {
	argv := Shell(infoScript, root)
	res, err := runner.Run(ctx, Quote(argv), nil)
	if err != nil {
		return nil, err
	}
	if err = ExitError(argv, res); err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n")
	if len(fields) < infoFields {
		return nil, errors.Wrapf(coretypes.ErrInvaildNodeEndpoint, "unexpected node info %q", res.Stdout)
	}
	ncpu, _ := strconv.Atoi(fields[1])
	memory, _ := strconv.ParseInt(fields[2], 10, 64)
	storage, _ := strconv.ParseInt(fields[3], 10, 64)
	return &enginetypes.Info{
		ID:           fields[0],
		NCPU:         ncpu,
		MemTotal:     memory * kiB,
		StorageTotal: storage * kiB,
	}, nil
}
