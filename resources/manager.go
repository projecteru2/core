package resources

import (
	"context"
	"math"

	"github.com/pkg/errors"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"golang.org/x/exp/slices"
)

// GetMostIdleNode .
func (pm *PluginsManager) GetMostIdleNode(ctx context.Context, nodenames []string) (string, error) {
	var mostIdleNode *GetMostIdleNodeResponse

	if len(nodenames) == 0 {
		return "", errors.Wrap(types.ErrGetMostIdleNodeFailed, "empty node names")
	}

	respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetMostIdleNodeResponse, error) {
		resp, err := plugin.GetMostIdleNode(ctx, nodenames)
		if err != nil {
			log.Errorf(ctx, "[GetMostIdleNode] plugin %v failed to get the most idle node of %v, err: %v", plugin.Name(), nodenames, err)
		}
		return resp, err
	})

	if err != nil {
		log.Errorf(ctx, "[GetMostIdleNode] failed to get the most idle node of %v", nodenames)
		return "", err
	}

	for _, resp := range respMap {
		if (mostIdleNode == nil || resp.Priority > mostIdleNode.Priority) && len(resp.NodeName) > 0 {
			mostIdleNode = resp
		}
	}

	if mostIdleNode == nil {
		return "", types.ErrGetMostIdleNodeFailed
	}
	return mostIdleNode.NodeName, nil
}

// GetNodesDeployCapacity returns available nodes which meet all the requirements
// the caller should require locks
// pure calculation
func (pm *PluginsManager) GetNodesDeployCapacity(ctx context.Context, nodenames []string, resourceOpts types.WorkloadResourceOpts) (map[string]*NodeCapacityInfo, int, error) {
	var res map[string]*NodeCapacityInfo

	respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetNodesDeployCapacityResponse, error) {
		resp, err := plugin.GetNodesDeployCapacity(ctx, nodenames, resourceOpts)
		if err != nil {
			log.Errorf(ctx, "[GetNodesDeployCapacity] plugin %v failed to get available nodenames, request %v, err %v", plugin.Name(), resourceOpts, err)
		}
		return resp, err
	})

	if err != nil {
		return nil, 0, err
	}

	// get nodenames with all resource capacities > 0
	for _, infoMap := range respMap {
		res = pm.mergeNodeCapacityInfo(res, infoMap.Nodes)
	}

	total := 0

	// weighted average
	for _, info := range res {
		info.Rate /= info.Weight
		info.Usage /= info.Weight
		if info.Capacity == math.MaxInt64 {
			total = math.MaxInt64
		} else {
			total += info.Capacity
		}
	}

	return res, total, nil
}

// SetNodeResourceCapacity updates node resource capacity
// receives resource options instead of resource args
func (pm *PluginsManager) SetNodeResourceCapacity(ctx context.Context, nodename string, nodeResourceOpts types.NodeResourceOpts, nodeResourceArgs map[string]types.NodeResourceArgs, delta bool, incr bool) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, error) {
	rollbackPlugins := []Plugin{}
	beforeMap := map[string]types.NodeResourceArgs{}
	afterMap := map[string]types.NodeResourceArgs{}

	return beforeMap, afterMap, utils.PCR(ctx,
		func(ctx context.Context) error {
			if nodeResourceArgs == nil {
				nodeResourceArgs = map[string]types.NodeResourceArgs{}
			}
			return nil
		},
		// commit: call plugins to set node resource
		func(ctx context.Context) error {
			respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*SetNodeResourceCapacityResponse, error) {
				resp, err := plugin.SetNodeResourceCapacity(ctx, nodename, nodeResourceOpts, nodeResourceArgs[plugin.Name()], delta, incr)
				if err != nil {
					log.Errorf(ctx, "[SetNodeResourceCapacity] node %v plugin %v failed to set node resource capacity, err: %v", nodename, plugin.Name(), err)
				}
				return resp, err
			})

			if err != nil {
				for plugin, resp := range respMap {
					rollbackPlugins = append(rollbackPlugins, plugin)
					beforeMap[plugin.Name()] = resp.Before
					afterMap[plugin.Name()] = resp.After
				}

				log.Errorf(ctx, "[SetNodeResourceCapacity] failed to set node resource for node %v", nodename)
				return err
			}
			return nil
		},
		// rollback: set the rollback resource args in reverse
		func(ctx context.Context) error {
			_, err := callPlugins(ctx, rollbackPlugins, func(plugin Plugin) (*SetNodeResourceCapacityResponse, error) {
				resp, err := plugin.SetNodeResourceCapacity(ctx, nodename, nil, beforeMap[plugin.Name()], false, false)
				if err != nil {
					log.Errorf(ctx, "[SetNodeResourceCapacity] node %v plugin %v failed to rollback node resource capacity, err: %v", err)
				}
				return resp, err
			})

			if err != nil {
				return err
			}
			return nil
		},
		pm.config.GlobalTimeout,
	)
}

// GetNodeResourceInfo .
func (pm *PluginsManager) GetNodeResourceInfo(ctx context.Context, nodename string, workloads []*types.Workload, fix bool) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, []string, error) {
	resourceCapacity := map[string]types.NodeResourceArgs{}
	resourceUsage := map[string]types.NodeResourceArgs{}
	resourceDiffs := []string{}

	plugins := pm.plugins
	if pm.config.ResourcePlugin.Whitelist != nil {
		plugins = utils.Filter(plugins, func(plugin Plugin) bool {
			return slices.Contains(pm.config.ResourcePlugin.Whitelist, plugin.Name())
		})
	}

	respMap, err := callPlugins(ctx, plugins, func(plugin Plugin) (*GetNodeResourceInfoResponse, error) {
		resp, err := plugin.GetNodeResourceInfo(ctx, nodename, workloads, fix)
		if err != nil {
			log.Errorf(ctx, "[GetNodeResourceInfo] plugin %v failed to get node resource of node %v, err: %v", plugin.Name(), nodename, err)
		}
		return resp, err
	})

	if err != nil {
		return nil, nil, nil, err
	}

	for plugin, resp := range respMap {
		resourceCapacity[plugin.Name()] = resp.ResourceInfo.Capacity
		resourceUsage[plugin.Name()] = resp.ResourceInfo.Usage
		resourceDiffs = append(resourceDiffs, resp.Diffs...)
	}

	return resourceCapacity, resourceUsage, resourceDiffs, nil
}

// SetNodeResourceUsage with rollback
func (pm *PluginsManager) SetNodeResourceUsage(ctx context.Context, nodename string, nodeResourceOpts types.NodeResourceOpts, nodeResourceArgs map[string]types.NodeResourceArgs, workloadResourceArgs []map[string]types.WorkloadResourceArgs, delta bool, incr bool) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, error) {
	workloadResourceArgsMap := map[string][]types.WorkloadResourceArgs{}
	rollbackPlugins := []Plugin{}
	beforeMap := map[string]types.NodeResourceArgs{}
	afterMap := map[string]types.NodeResourceArgs{}

	return beforeMap, afterMap, utils.PCR(ctx,
		// prepare: convert []map[plugin]resourceArgs to map[plugin][]resourceArgs
		// [{"cpu-plugin": {"cpu": 1}}, {"cpu-plugin": {"cpu": 1}}] -> {"cpu-plugin": [{"cpu": 1}, {"cpu": 1}]}
		func(ctx context.Context) error {
			for _, workloadResourceArgs := range workloadResourceArgs {
				for plugin, rawParams := range workloadResourceArgs {
					if _, ok := workloadResourceArgsMap[plugin]; !ok {
						workloadResourceArgsMap[plugin] = []types.WorkloadResourceArgs{}
					}
					workloadResourceArgsMap[plugin] = append(workloadResourceArgsMap[plugin], rawParams)
				}
			}
			if nodeResourceArgs == nil {
				nodeResourceArgs = map[string]types.NodeResourceArgs{}
			}
			return nil
		},
		// commit: call plugins to set node resource
		func(ctx context.Context) error {
			respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*SetNodeResourceUsageResponse, error) {
				resp, err := plugin.SetNodeResourceUsage(ctx, nodename, nodeResourceOpts, nodeResourceArgs[plugin.Name()], workloadResourceArgsMap[plugin.Name()], delta, incr)
				if err != nil {
					log.Errorf(ctx, "[SetNodeResourceUsage] node %v plugin %v failed to update node resource, err: %v", nodename, plugin.Name(), err)
				}
				return resp, err
			})

			if err != nil {
				for plugin, resp := range respMap {
					rollbackPlugins = append(rollbackPlugins, plugin)
					beforeMap[plugin.Name()] = resp.Before
					afterMap[plugin.Name()] = resp.After
				}

				log.Errorf(ctx, "[UpdateNodeResourceUsage] failed to set node resource for node %v", nodename)
				return err
			}
			return nil
		},
		// rollback: set the rollback resource args in reverse
		func(ctx context.Context) error {
			_, err := callPlugins(ctx, rollbackPlugins, func(plugin Plugin) (*SetNodeResourceUsageResponse, error) {
				resp, err := plugin.SetNodeResourceUsage(ctx, nodename, nil, beforeMap[plugin.Name()], nil, false, false)
				if err != nil {
					log.Errorf(ctx, "[UpdateNodeResourceUsage] node %v plugin %v failed to rollback node resource, err: %v", err)
				}
				return resp, err
			})

			if err != nil {
				return err
			}
			return nil
		},
		pm.config.GlobalTimeout,
	)
}

// Alloc .
func (pm *PluginsManager) Alloc(ctx context.Context, nodename string, deployCount int, resourceOpts types.WorkloadResourceOpts) ([]types.EngineArgs, []map[string]types.WorkloadResourceArgs, error) {
	resEngineArgs := make([]types.EngineArgs, deployCount)
	resResourceArgs := make([]map[string]types.WorkloadResourceArgs, deployCount)

	// init engine args
	for i := 0; i < deployCount; i++ {
		resEngineArgs[i] = types.EngineArgs{}
		resResourceArgs[i] = map[string]types.WorkloadResourceArgs{}
	}

	return resEngineArgs, resResourceArgs, utils.PCR(ctx,
		// prepare: calculate engine args and resource args
		func(ctx context.Context) error {
			respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetDeployArgsResponse, error) {
				resp, err := plugin.GetDeployArgs(ctx, nodename, deployCount, resourceOpts)
				if err != nil {
					log.Errorf(ctx, "[Alloc] plugin %v failed to compute alloc args, request %v, node %v, deploy count %v, err %v", plugin.Name(), resourceOpts, nodename, deployCount, err)
				}
				return resp, err
			})
			if err != nil {
				return err
			}

			// calculate engine args
			for plugin, resp := range respMap {
				for index, args := range resp.ResourceArgs {
					resResourceArgs[index][plugin.Name()] = args
				}
				for index, args := range resp.EngineArgs {
					resEngineArgs[index], err = pm.mergeEngineArgs(ctx, resEngineArgs[index], args)
					if err != nil {
						log.Errorf(ctx, "[Alloc] invalid engine args")
						return err
					}
				}
			}
			return nil
		},
		// commit: update node resources
		func(ctx context.Context) error {
			if _, _, err := pm.SetNodeResourceUsage(ctx, nodename, nil, nil, resResourceArgs, true, Incr); err != nil {
				log.Errorf(ctx, "[Alloc] failed to update node resource, err: %v", err)
				return err
			}
			return nil
		},
		// rollback: do nothing
		func(ctx context.Context) error {
			return nil
		},
		pm.config.GlobalTimeout,
	)
}

// RollbackAlloc rollbacks the allocated resource
func (pm *PluginsManager) RollbackAlloc(ctx context.Context, nodename string, resourceArgs []map[string]types.WorkloadResourceArgs) error {
	_, _, err := pm.SetNodeResourceUsage(ctx, nodename, nil, nil, resourceArgs, true, Decr)
	return err
}

// Realloc reallocates resource for workloads, returns engine args and final resource args.
func (pm *PluginsManager) Realloc(ctx context.Context, nodename string, originResourceArgs map[string]types.WorkloadResourceArgs, resourceOpts types.WorkloadResourceOpts) (types.EngineArgs, map[string]types.WorkloadResourceArgs, map[string]types.WorkloadResourceArgs, error) {
	resEngineArgs := types.EngineArgs{}
	resDeltaResourceArgs := map[string]types.WorkloadResourceArgs{}
	resFinalResourceArgs := map[string]types.WorkloadResourceArgs{}

	return resEngineArgs, resDeltaResourceArgs, resFinalResourceArgs, utils.PCR(ctx,
		// prepare: calculate engine args, delta node resource args and final workload resource args
		func(ctx context.Context) error {
			respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetReallocArgsResponse, error) {
				resp, err := plugin.GetReallocArgs(ctx, nodename, originResourceArgs[plugin.Name()], resourceOpts)
				if err != nil {
					log.Errorf(ctx, "[Realloc] plugin %v failed to calculate realloc args, err: %v", plugin.Name(), err)
				}
				return resp, err
			})

			if err != nil {
				log.Errorf(ctx, "[Realloc] realloc failed, origin: %+v, opts: %+v", originResourceArgs, resourceOpts)
				return err
			}

			for plugin, resp := range respMap {
				if resEngineArgs, err = pm.mergeEngineArgs(ctx, resEngineArgs, resp.EngineArgs); err != nil {
					log.Errorf(ctx, "[Realloc] invalid engine args, err: %v", err)
					return err
				}
				resDeltaResourceArgs[plugin.Name()] = resp.Delta
				resFinalResourceArgs[plugin.Name()] = resp.ResourceArgs
			}
			return nil
		},
		// commit: update node resource
		func(ctx context.Context) error {
			if _, _, err := pm.SetNodeResourceUsage(ctx, nodename, nil, nil, []map[string]types.WorkloadResourceArgs{resDeltaResourceArgs}, true, Incr); err != nil {
				log.Errorf(ctx, "[Realloc] failed to update nodename resource, err: %v", err)
				return err
			}
			return nil
		},
		// rollback: do nothing
		func(ctx context.Context) error {
			return nil
		},
		pm.config.GlobalTimeout,
	)
}

// RollbackRealloc rollbacks the resource changes caused by realloc
func (pm *PluginsManager) RollbackRealloc(ctx context.Context, nodename string, resourceArgs map[string]types.WorkloadResourceArgs) error {
	_, _, err := pm.SetNodeResourceUsage(ctx, nodename, nil, nil, []map[string]types.WorkloadResourceArgs{resourceArgs}, true, Decr)
	return err
}

// GetMetricsDescription .
func (pm *PluginsManager) GetMetricsDescription(ctx context.Context) ([]*MetricsDescription, error) {
	var metricsDescriptions []*MetricsDescription
	respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetMetricsDescriptionResponse, error) {
		resp, err := plugin.GetMetricsDescription(ctx)
		if err != nil {
			log.Errorf(ctx, "[GetMetricsDescription] plugin %v failed to get metrics description, err: %v", plugin.Name(), err)
		}
		return resp, err
	})

	if err != nil {
		log.Errorf(ctx, "[GetMetricsDescription] failed to get metrics description")
		return nil, err
	}

	for _, resp := range respMap {
		metricsDescriptions = append(metricsDescriptions, *resp...)
	}

	return metricsDescriptions, nil
}

// GetNodeMetrics .
func (pm *PluginsManager) GetNodeMetrics(ctx context.Context, node *types.Node) ([]*Metrics, error) {
	var metrics []*Metrics
	respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetNodeMetricsResponse, error) {
		capacity, usage := node.Resource.Capacity[plugin.Name()], node.Resource.Usage[plugin.Name()]
		resp, err := plugin.GetNodeMetrics(ctx, node.Podname, node.Name, &NodeResourceInfo{Capacity: capacity, Usage: usage})
		if err != nil {
			log.Errorf(ctx, "[GetNodeMetrics] plugin %v failed to convert node resource info to metrics, err: %v", plugin.Name(), err)
		}
		return resp, err
	})

	if err != nil {
		log.Errorf(ctx, "[GetNodeMetrics] failed to convert node resource info to metrics")
		return nil, err
	}

	for _, resp := range respMap {
		metrics = append(metrics, *resp...)
	}

	return metrics, nil
}

// AddNode .
func (pm *PluginsManager) AddNode(ctx context.Context, nodename string, resourceOpts types.NodeResourceOpts, nodeInfo *enginetypes.Info) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, error) {
	resResourceCapacity := map[string]types.NodeResourceArgs{}
	resResourceUsage := map[string]types.NodeResourceArgs{}
	rollbackPlugins := []Plugin{}

	return resResourceCapacity, resResourceUsage, utils.PCR(ctx,
		// prepare: do nothing
		func(ctx context.Context) error {
			return nil
		},
		// commit: call plugins to add the node
		func(ctx context.Context) error {
			respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*AddNodeResponse, error) {
				resp, err := plugin.AddNode(ctx, nodename, resourceOpts, nodeInfo)
				if err != nil {
					log.Errorf(ctx, "[AddNode] node %v plugin %v failed to add node, req: %v, err: %v", nodename, plugin.Name(), resourceOpts, err)
				}
				return resp, err
			})

			if err != nil {
				for plugin := range respMap {
					rollbackPlugins = append(rollbackPlugins, plugin)
				}

				log.Errorf(ctx, "[AddNode] node %v failed to add node %v, rollback", nodename, resourceOpts)
				return err
			}

			for plugin, resp := range respMap {
				resResourceCapacity[plugin.Name()] = resp.Capacity
				resResourceUsage[plugin.Name()] = resp.Usage
			}

			return nil
		},
		// rollback: remove node
		func(ctx context.Context) error {
			_, err := callPlugins(ctx, rollbackPlugins, func(plugin Plugin) (*RemoveNodeResponse, error) {
				resp, err := plugin.RemoveNode(ctx, nodename)
				if err != nil {
					log.Errorf(ctx, "[AddNode] node %v plugin %v failed to rollback, err: %v", nodename, plugin.Name(), err)
				}
				return resp, err
			})

			if err != nil {
				log.Errorf(ctx, "[AddNode] failed to rollback")
				return err
			}

			return nil
		},
		pm.config.GlobalTimeout,
	)
}

// RemoveNode .
func (pm *PluginsManager) RemoveNode(ctx context.Context, nodename string) error {
	var resourceCapacityMap map[string]types.NodeResourceArgs
	var resourceUsageMap map[string]types.NodeResourceArgs
	rollbackPlugins := []Plugin{}

	return utils.PCR(ctx,
		// prepare: get node resource
		func(ctx context.Context) error {
			var err error
			resourceCapacityMap, resourceUsageMap, _, err = pm.GetNodeResourceInfo(ctx, nodename, nil, false)
			if err != nil {
				log.Errorf(ctx, "[RemoveNode] failed to get node %v resource, err: %v", nodename, err)
				return err
			}
			return nil
		},
		// commit: remove node
		func(ctx context.Context) error {
			respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*RemoveNodeResponse, error) {
				resp, err := plugin.RemoveNode(ctx, nodename)
				if err != nil {
					log.Errorf(ctx, "[AddNode] plugin %v failed to remove node, err: %v", plugin.Name(), nodename, err)
				}
				return resp, err
			})

			if err != nil {
				for plugin := range respMap {
					rollbackPlugins = append(rollbackPlugins, plugin)
				}

				log.Errorf(ctx, "[AddNode] failed to remove node %v", nodename)
				return err
			}
			return nil
		},
		// rollback: add node
		func(ctx context.Context) error {
			_, err := callPlugins(ctx, rollbackPlugins, func(plugin Plugin) (*SetNodeResourceInfoResponse, error) {
				resp, err := plugin.SetNodeResourceInfo(ctx, nodename, resourceCapacityMap[plugin.Name()], resourceUsageMap[plugin.Name()])
				if err != nil {
					log.Errorf(ctx, "[RemoveNode] plugin %v node %v failed to rollback, err: %v", plugin.Name(), nodename, err)
				}
				return resp, err
			})

			if err != nil {
				log.Errorf(ctx, "[RemoveNode] failed to rollback")
				return err
			}
			return nil
		},
		pm.config.GlobalTimeout,
	)
}

// GetRemapArgs remaps resource and returns engine args for workloads. format: {"workload-1": {"cpus": ["1-3"]}}
// remap doesn't change resource args
func (pm *PluginsManager) GetRemapArgs(ctx context.Context, nodename string, workloadMap map[string]*types.Workload) (map[string]types.EngineArgs, error) {
	resEngineArgsMap := map[string]types.EngineArgs{}

	// call plugins to remap
	respMap, err := callPlugins(ctx, pm.plugins, func(plugin Plugin) (*GetRemapArgsResponse, error) {
		resp, err := plugin.GetRemapArgs(ctx, nodename, workloadMap)
		if err != nil {
			log.Errorf(ctx, "[GetRemapArgs] plugin %v node %v failed to remap, err: %v", plugin.Name(), nodename, err)
		}
		return resp, err
	})

	if err != nil {
		return nil, err
	}

	// merge engine args
	for _, resp := range respMap {
		for workloadID, engineArgs := range resp.EngineArgsMap {
			if _, ok := resEngineArgsMap[workloadID]; !ok {
				resEngineArgsMap[workloadID] = types.EngineArgs{}
			}
			resEngineArgsMap[workloadID], err = pm.mergeEngineArgs(ctx, resEngineArgsMap[workloadID], engineArgs)
			if err != nil {
				log.Errorf(ctx, "[GetRemapArgs] invalid engine args")
				return nil, err
			}
		}
	}

	return resEngineArgsMap, nil
}

func (pm *PluginsManager) mergeNodeCapacityInfo(m1 map[string]*NodeCapacityInfo, m2 map[string]*NodeCapacityInfo) map[string]*NodeCapacityInfo {
	if m1 == nil {
		return m2
	}

	res := map[string]*NodeCapacityInfo{}
	for node, info1 := range m1 {
		// all the capacities should > 0
		if info2, ok := m2[node]; ok {
			res[node] = &NodeCapacityInfo{
				NodeName: node,
				Capacity: utils.Min(info1.Capacity, info2.Capacity),
				Rate:     info1.Rate + info2.Rate*info2.Weight,
				Usage:    info1.Usage + info2.Usage*info2.Weight,
				Weight:   info1.Weight + info2.Weight,
			}
		}
	}
	return res
}

// mergeEngineArgs e.g. {"file": ["/bin/sh:/bin/sh"], "cpu": 1.2, "cpu-bind": true} + {"file": ["/bin/ls:/bin/ls"], "mem": "1PB"}
// => {"file": ["/bin/sh:/bin/sh", "/bin/ls:/bin/ls"], "cpu": 1.2, "cpu-bind": true, "mem": "1PB"}
func (pm *PluginsManager) mergeEngineArgs(ctx context.Context, m1 types.EngineArgs, m2 types.EngineArgs) (types.EngineArgs, error) {
	res := types.EngineArgs{}
	for key, value := range m1 {
		res[key] = value
	}
	for key, value := range m2 {
		if _, ok := res[key]; ok {
			// only two string slices can be merged
			_, ok1 := res[key].([]string)
			_, ok2 := value.([]string)
			if !ok1 || !ok2 {
				log.Errorf(ctx, "[mergeEngineArgs] only two string slices can be merged! error key %v, m1[key] = %v, m2[key] = %v", key, m1[key], m2[key])
				return nil, types.ErrInvalidEngineArgs
			}
			res[key] = append(res[key].([]string), value.([]string)...)
		} else {
			res[key] = value
		}
	}
	return res, nil
}
