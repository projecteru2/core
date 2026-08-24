package systemd

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

func (e *Engine) NetworkConnect(_ context.Context, _, _, _, _ string) (subnets []string, err error) {
	err = types.ErrEngineNotImplemented
	return subnets, err
}

func (e *Engine) NetworkDisconnect(_ context.Context, _, _ string, _ bool) (err error) {
	err = types.ErrEngineNotImplemented
	return err
}

func (e *Engine) NetworkList(_ context.Context, _ []string) (networks []*enginetypes.Network, err error) {
	err = types.ErrEngineNotImplemented
	return networks, err
}
