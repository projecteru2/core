package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/utils"
)

/*
Alloc alloc resource
opts struct

	{
		"plugin1":{
			"cpu-request": 1.2,
			"cpu-limit": 2.0,
		},
		"plugin2":{
		},
	}
*/
func (m Manager) Alloc(ctx context.Context, nodename string, deployCount int, opts resourcetypes.Resources) ([]resourcetypes.Resources, []resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.coblat.Alloc")

	// index -> no, map by plugin name
	workloadsParams := make([]resourcetypes.Resources, deployCount)
	engineParams := make([]resourcetypes.Resources, deployCount)

	// init engine args
	for i := 0; i < deployCount; i++ {
		workloadsParams[i] = resourcetypes.Resources{}
		engineParams[i] = resourcetypes.Resources{}
	}

	return workloadsParams, engineParams, utils.PCR(ctx,
		// prepare: calculate engine args and resource args
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

			// calculate engine args
			for plugin, resp := range resps {
				logger.Debug(ctx, plugin.Name())
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
		// commit: update node resources
		func(ctx context.Context) error {
			// 因为用了资源所以 usage 增加
			if _, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, workloadsParams, true, plugins.Incr); err != nil {
				logger.Error(ctx, err, "failed to update node resource")
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

// RollbackAlloc rollbacks the allocated resource
func (m Manager) RollbackAlloc(ctx context.Context, nodename string, workloadsParams []resourcetypes.Resources) error {
	_, _, err := m.SetNodeResourceUsage(ctx, nodename, nil, nil, workloadsParams, true, plugins.Decr)
	return err
}
