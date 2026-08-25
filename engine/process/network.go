package process

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

func (e *Engine) NetworkConnect(context.Context, string, string, string, string) ([]string, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) NetworkDisconnect(context.Context, string, string, bool) error {
	return coretypes.ErrEngineNotImplemented
}

func (e *Engine) NetworkList(context.Context, []string) ([]*enginetypes.Network, error) {
	return nil, coretypes.ErrEngineNotImplemented
}
