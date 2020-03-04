package systemd

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

func (s *SystemdSSH) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) NetworkDisconnect(ctx context.Context, network, target string, force bool) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

func (s *SystemdSSH) NetworkList(ctx context.Context, driver []string) (networks []*enginetypes.Network, err error) {
	err = types.ErrEngineNotImplemented
	return
}
