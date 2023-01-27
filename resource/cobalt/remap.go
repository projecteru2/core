package cobalt

import (
	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	"github.com/projecteru2/core/types"
)

// Remap remaps resource and returns engine args for workloads. format: {"workload-1": {"cpus": ["1-3"]}}
// remap doesn't change resource args
func (m Manager) Remap(ctx context.Context, nodename string, workloads []*types.Workload) (*types.Resources, error) {
	logger := log.WithFunc("resource.cobalt.GetRemapArgs").WithField("node", nodename)
	// call plugins to remap
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.CalculateRemapResponse, error) {
		workloadsResourceMap := map[string]*plugintypes.WorkloadResource{}
		for _, workload := range workloads {
			workloadsResourceMap[workload.ID] = (*workload.Resources)[plugin.Name()]
		}
		resp, err := plugin.CalculateRemap(ctx, nodename, workloadsResourceMap)
		if err != nil {
			logger.Errorf(ctx, err, "plugin %+v node %+v failed to remap", plugin.Name(), nodename)
		}
		return resp, err
	})
	if err != nil {
		return nil, err
	}

	enginesParams := &types.Resources{}
	// merge engine args
	for _, resp := range resps {
		for workloadID, engineParams := range resp.EngineParamsMap {
			if _, ok := (*enginesParams)[workloadID]; !ok {
				(*enginesParams)[workloadID] = &types.RawParams{}
			}
			v := (*enginesParams)[workloadID]
			vMerged, err := m.mergeEngineParams(ctx, v, engineParams)
			if err != nil {
				logger.Error(ctx, err, "invalid engine args")
				return nil, err
			}
			(*enginesParams)[workloadID] = vMerged
		}
	}

	return enginesParams, nil
}

// mergeEngineParams e.g. {"file": ["/bin/sh:/bin/sh"], "cpu": 1.2, "cpu-bind": true} + {"file": ["/bin/ls:/bin/ls"], "mem": "1PB"}
// => {"file": ["/bin/sh:/bin/sh", "/bin/ls:/bin/ls"], "cpu": 1.2, "cpu-bind": true, "mem": "1PB"}
func (m Manager) mergeEngineParams(ctx context.Context, m1 *plugintypes.EngineParams, m2 *plugintypes.EngineParams) (*plugintypes.EngineParams, error) {
	r := &plugintypes.EngineParams{}
	for key, value := range *m1 {
		(*r)[key] = value
	}
	for key, value := range *m2 {
		if _, ok := (*r)[key]; ok {
			// only two string slices can be merged
			_, ok1 := (*r)[key].([]string)
			_, ok2 := value.([]string)
			if !ok1 || !ok2 {
				log.WithFunc("resource.cobalt.mergeEngineParams").Errorf(ctx, types.ErrInvalidEngineArgs, "only two string slices can be merged! error key %+v, m1[key] = %+v, m2[key] = %+v", key, (*m1)[key], (*m2)[key])
				return nil, types.ErrInvalidEngineArgs
			}
			(*r)[key] = append((*r)[key].([]string), value.([]string)...)
		} else {
			(*r)[key] = value
		}
	}
	return r, nil
}
