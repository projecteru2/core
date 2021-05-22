package systemd

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// NetworkConnect connects target netloc
func (e *Engine) NetworkConnect(ctx context.Context, network, target, ipv4, ipv6 string) (subnets []string, err error) {
	err = types.ErrEngineNotImplemented
	return
}

// NetworkDisconnect disconnects target netloc
func (e *Engine) NetworkDisconnect(ctx context.Context, network, target string, force bool) (err error) {
	err = types.ErrEngineNotImplemented
	return
}

// NetworkList lists networks
func (e *Engine) NetworkList(ctx context.Context, driver []string) (networks []*enginetypes.Network, err error) {
	err = types.ErrEngineNotImplemented
	return
}
