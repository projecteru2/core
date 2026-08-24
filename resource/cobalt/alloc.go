package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/utils"
)

// Alloc allocates resource for deployCount workloads on nodename, keyed by plugin name in opts.
func (m Manager) Alloc(ctx context.Context, nodename string, deployCount int, opts resourcetypes.Resources) ([]resourcetypes.Resources, []resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.cobalt.Alloc")

	workloadsParams := make([]resourcetypes.Resources, deployCount)
	engineParams := make([]resourcetypes.Resources, deployCount)

	for i := range deployCount {
		workloadsParams[i] = resourcetypes.Resources{}
		engineParams[i] = resourcetypes.Resources{}
	}

	return workloadsParams, engineParams, utils.PCR(ctx,
		func(ctx context.Context) error {
			resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.CalculateDeployResponse, error) {
				resp, err := plugin.CalculateDeploy(ctx, nodename, deployCount, opts[plugin.Name()])
				if err != nil {
					logger.Errorf(ctx, err, "plugin %+v failed to compute alloc args, request %+v, node %+v, deploy count %+v", plugin.Name(), opts, nodename, deployCount)
				}
				return resp, err
			})
			if err != nil {
				return err
			}

			for plugin, resp := range resps {
				logger.Debugf(ctx, "plugin %s calculated deploy", plugin.Name())
				for index, params := range resp.WorkloadsResource {
					workloadsParams[index][plugin.Name()] = params
				}
				for index, params := range resp.EnginesParams {
					v, err := m.mergeEngineParams(ctx, engineParams[index][plugin.Name()], params)
					if err != nil {
						logger.Error(ctx, err, "invalid engine args")
						return err
					}
					engineParams[index][plugin.Name()] = v
				}
			}
			return nil
		},
		func(ctx context.Context) error {
			if _, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, workloadsParams, true, plugins.Incr); err != nil {
				logger.Error(ctx, err, "failed to update node resource")
				return err
			}
			return nil
		},
		func(_ context.Context) error {
			return nil
		},
		m.config.GlobalTimeout,
	)
}

// RollbackAlloc returns the allocated resource to the node.
func (m Manager) RollbackAlloc(ctx context.Context, nodename string, workloadsParams []resourcetypes.Resources) error {
	_, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, workloadsParams, true, plugins.Decr)
	return err
}
