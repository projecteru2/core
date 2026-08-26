package containerd

import (
	"context"
	"slices"

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
	return cni.Select(res.Stdout, func(c cni.Conf) bool { return drives(c, drivers) })
}

// drives reports whether one of the named plugin types implements the conf.
func drives(c cni.Conf, drivers []string) bool {
	if len(drivers) == 0 {
		return true
	}
	if slices.Contains(drivers, c.Type) {
		return true
	}
	return slices.ContainsFunc(c.Plugins, func(plugin cni.Conf) bool {
		return slices.Contains(drivers, plugin.Type)
	})
}
