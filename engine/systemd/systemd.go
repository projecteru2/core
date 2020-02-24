package systemd

import (
	"context"

	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	SSHPrefixKey = "systemd://"
)

type SystemdSSH struct{}

func MakeClient(ctx context.Context, config coretypes.Config, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	return &SystemdSSH{}, nil
}

func (s *SystemdSSH) Info(ctx context.Context) (info *enginetypes.Info, err error) {
	return
}

func (s *SystemdSSH) ResourceValidate(ctx context.Context, cpu float64, cpumap map[string]int64, memory, storage int64) (err error) {
	return
}
