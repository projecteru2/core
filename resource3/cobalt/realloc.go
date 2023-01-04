package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource3/plugins"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (m Manager) Realloc(ctx context.Context, nodename string, nodeResource types.Resources, opts types.Resources) (*plugintypes.EngineParams, types.Resources, types.Resources, error) {
	logger := log.WithFunc("resource.cobalt.Realloc").WithField("node", nodename)
	engineParams := &plugintypes.EngineParams{}
	deltaResources := types.Resources{}
	workloadResource := types.Resources{}

	return engineParams, deltaResources, workloadResource, utils.PCR(ctx,
		// prepare: calculate engine args, delta node resource args and final workload resource args
		func(ctx context.Context) error {
			resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.CalculateReallocResponse, error) {
				resp, err := plugin.CalculateRealloc(ctx, nodename, nodeResource[plugin.Name()], opts[plugin.Name()])
				if err != nil {
					logger.Errorf(ctx, err, "plugin %+v failed to calculate realloc args", plugin.Name())
				}
				return resp, err
			})
			if err != nil {
				logger.Errorf(ctx, err, "realloc failed, origin: %+v, opts: %+v", nodeResource, opts)
				return err
			}

			for plugin, resp := range resps {
				if engineParams, err = m.mergeEngineParams(ctx, engineParams, resp.EngineParams); err != nil {
					logger.Error(ctx, err, "invalid engine args")
					return err
				}
				deltaResources[plugin.Name()] = resp.DeltaResource
				workloadResource[plugin.Name()] = resp.WorkloadResource
			}
			return nil
		},
		// commit: update node resource
		func(ctx context.Context) error {
			// TODO 存在问题？？3个参数是完整的变化，差值变化，按照 workloads 的变化
			if _, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, []types.Resources{workloadResource}, true, plugins.Incr); err != nil {
				logger.Error(ctx, err, "failed to update nodename resource")
				return err
			}
			return nil
		},
		// rollback: do nothing
		func(ctx context.Context) error {
			return nil
		},
		m.config.GlobalTimeout,
	)
}

func (m Manager) RollbackRealloc(ctx context.Context, nodename string, workloadParams types.Resources) error {
	_, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, []types.Resources{workloadParams}, true, plugins.Decr)
	return err
}
