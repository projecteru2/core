package containerd

import (
	"context"

	"github.com/projecteru2/core/engine/cni"
	"github.com/projecteru2/core/engine/sshrunner"
	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

// NetworkConnect is not a CNI operation: a network is attached when the netns is created.
func (e *Engine) NetworkConnect(context.Context, string, string, string, string) ([]string, error) {
	return nil, coretypes.ErrEngineNotImplemented
}

func (e *Engine) NetworkDisconnect(context.Context, string, string, bool) error {
	return coretypes.ErrEngineNotImplemented
}

func (e *Engine) NetworkList(ctx context.Context, drivers []string) ([]*enginetypes.Network, error) {
	res, err := e.run(ctx, sshrunner.Shell(cni.ListScript, cni.ConfDir)...)
	if err != nil {
		return nil, err
	}
	return cni.Parse(res.Stdout, drivers)
}
