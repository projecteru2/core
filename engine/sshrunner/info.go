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

	infoScript = `set -e
mkdir -p "$1"
id=$(cat /etc/machine-id)
ncpu=$(nproc)
memory=$(awk '/^MemTotal:/{print $2}' /proc/meminfo)
storage=$(df -Pk "$1" | awk 'NR==2{print $2}')
printf '%s\n' "$id" "$ncpu" "$memory" "$storage"
`
)

func NodeInfo(ctx context.Context, runner Runner, root string) (*enginetypes.Info, error) {
	res, err := Run(ctx, runner, Shell(infoScript, root)...)
	if err != nil {
		return nil, err
	}
	fields := strings.Split(strings.TrimRight(res.Stdout, "\n"), "\n")
	if len(fields) < infoFields {
		return nil, errors.Wrapf(coretypes.ErrInvaildNodeEndpoint, "unexpected node info %q", res.Stdout)
	}
	ncpu, cpuErr := strconv.Atoi(fields[1])
	memory, memErr := strconv.ParseInt(fields[2], 10, 64)
	storage, storageErr := strconv.ParseInt(fields[3], 10, 64)
	if errors.Join(cpuErr, memErr, storageErr) != nil {
		return nil, errors.Wrapf(coretypes.ErrInvaildNodeEndpoint, "unexpected node info %q", res.Stdout)
	}
	return &enginetypes.Info{
		ID:           fields[0],
		NCPU:         ncpu,
		MemTotal:     memory * kiB,
		StorageTotal: storage * kiB,
	}, nil
}
