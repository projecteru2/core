package cobalt

import (
	"context"
	"maps"

	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

// Remap returns engine params per workload, format: {"workload-1": {"cpus": ["1-3"]}}.
// remap never changes resource params
func (m *Manager) Remap(ctx context.Context, nodename string, workloads []*types.Workload) (map[string]resourcetypes.Resources, error) {
	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.CalculateRemapResponse, error) {
		name := plugin.Name()
		workloadsResourceMap := make(map[string]plugintypes.WorkloadResource, len(workloads))
		for _, workload := range workloads {
			workloadsResourceMap[workload.ID] = workload.Resources[name]
		}
		return plugin.CalculateRemap(ctx, nodename, workloadsResourceMap)
	})
	if err != nil {
		return nil, err
	}

	enginesParams := map[string]resourcetypes.Resources{}
	for plugin, resp := range resps {
		name := plugin.Name()
		for workloadID, engineParams := range resp.EngineParamsMap {
			if _, ok := enginesParams[workloadID]; !ok {
				enginesParams[workloadID] = resourcetypes.Resources{}
			}
			enginesParams[workloadID][name] = maps.Clone(engineParams)
		}
	}

	return enginesParams, nil
}
