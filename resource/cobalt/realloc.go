package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/utils"
)

// Realloc .
func (m Manager) Realloc(ctx context.Context, nodename string, nodeResource resourcetypes.Resources, opts resourcetypes.Resources) (resourcetypes.Resources, resourcetypes.Resources, resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.cobalt.Realloc").WithField("node", nodename)
	engineParams := resourcetypes.Resources{}
	deltaResources := resourcetypes.Resources{}
	workloadResource := resourcetypes.Resources{}

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
				v, err := m.mergeEngineParams(ctx, engineParams[plugin.Name()], resp.EngineParams)
				if err != nil {
					logger.Error(ctx, err, "invalid engine args")
					return err
				}
				engineParams[plugin.Name()] = v
				deltaResources[plugin.Name()] = resp.DeltaResource
				workloadResource[plugin.Name()] = resp.WorkloadResource
			}
			return nil
		},
		// commit: update node resource
		func(ctx context.Context) error {
			// TODO 存在问题？？3个参数是完整的变化，差值变化，按照 workloads 的变化
			if _, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, []resourcetypes.Resources{deltaResources}, true, plugins.Incr); err != nil {
				logger.Error(ctx, err, "failed to update nodename resource")
				return err
			}
			return nil
		},
		// rollback: do nothing
		func(_ context.Context) error {
			return nil
		},
		m.config.GlobalTimeout,
	)
}

// RollbackRealloc .
func (m Manager) RollbackRealloc(ctx context.Context, nodename string, workloadParams resourcetypes.Resources) error {
	_, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, []resourcetypes.Resources{workloadParams}, true, plugins.Decr)
	return err
}
