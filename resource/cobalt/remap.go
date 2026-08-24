package cobalt

import (
	"context"
	"maps"
	"slices"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

// Remap returns engine params per workload, format: {"workload-1": {"cpus": ["1-3"]}}.
// remap never changes resource params
func (m Manager) Remap(ctx context.Context, nodename string, workloads []*types.Workload) (map[string]resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.cobalt.Remap").WithField("node", nodename)
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.CalculateRemapResponse, error) {
		workloadsResourceMap := map[string]plugintypes.WorkloadResource{}
		for _, workload := range workloads {
			workloadsResourceMap[workload.ID] = workload.Resources[plugin.Name()]
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

	enginesParams := map[string]resourcetypes.Resources{}
	for plugin, resp := range resps {
		for workloadID, engineParams := range resp.EngineParamsMap {
			if _, ok := enginesParams[workloadID]; !ok {
				enginesParams[workloadID] = resourcetypes.Resources{plugin.Name(): resourcetypes.RawParams{}}
			}
			v, err := m.mergeEngineParams(ctx, enginesParams[workloadID][plugin.Name()], engineParams)
			if err != nil {
				logger.Error(ctx, err, "invalid engine args")
				return nil, err
			}
			enginesParams[workloadID][plugin.Name()] = v
		}
	}

	return enginesParams, nil
}

// mergeEngineParams concatenates string-slice values and takes m2 for keys absent from m1.
func (m Manager) mergeEngineParams(ctx context.Context, m1, m2 plugintypes.EngineParams) (plugintypes.EngineParams, error) {
	r := plugintypes.EngineParams{}
	maps.Copy(r, m1)
	for key, value := range m2 {
		old, ok := r[key]
		if !ok {
			r[key] = value
			continue
		}
		s1, ok1 := old.([]string)
		s2, ok2 := value.([]string)
		if !ok1 || !ok2 {
			log.WithFunc("resource.cobalt.mergeEngineParams").Errorf(ctx, types.ErrInvalidEngineArgs, "only string slices can be merged, key %+v, m1 %+v, m2 %+v", key, m1[key], m2[key])
			return nil, types.ErrInvalidEngineArgs
		}
		r[key] = slices.Concat(s1, s2)
	}
	return r, nil
}
