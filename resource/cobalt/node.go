package cobalt

import (
	"context"
	"maps"
	"math"
	"slices"

	"github.com/cockroachdb/errors"
	"github.com/sanity-io/litter"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (m Manager) AddNode(ctx context.Context, nodename string, opts resourcetypes.Resources, nodeInfo *enginetypes.Info) (resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.cobalt.AddNode").WithField("node", nodename)
	res := resourcetypes.Resources{}
	rollbackPlugins := []plugins.Plugin{}

	return res, utils.PCR(ctx,
		nil,
		func(ctx context.Context) error {
			resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.AddNodeResponse, error) {
				r := opts[plugin.Name()]
				// plugins run even for a nil request: they read config from engine info and seed an empty etcd entry
				logger.WithField("plugin", plugin.Name()).Debugf(ctx, "add node request %v", litter.Sdump(r))
				resp, err := plugin.AddNode(ctx, nodename, r, nodeInfo)
				if err != nil {
					logger.Errorf(ctx, err, "node %+v plugin %+v failed to add node, req: %+v", nodename, plugin.Name(), litter.Sdump(r))
				}
				return resp, err
			})
			if err != nil {
				rollbackPlugins = slices.Collect(maps.Keys(resps))
				return err
			}

			for plugin, resp := range resps {
				res[plugin.Name()] = resp.Capacity
			}
			return nil
		},
		func(ctx context.Context) error {
			err := rollbackNodeResource(ctx, logger, nodename, rollbackPlugins, func(plugin plugins.Plugin) (*plugintypes.RemoveNodeResponse, error) {
				return plugin.RemoveNode(ctx, nodename)
			})
			if err != nil {
				logger.Error(ctx, err, "failed to rollback")
			}
			return err
		},
		m.config.GlobalTimeout,
	)
}

func (m Manager) RemoveNode(ctx context.Context, nodename string) error {
	logger := log.WithFunc("resource.cobalt.RemoveNode").WithField("node", nodename)
	var nodeCapacity resourcetypes.Resources
	var nodeUsage resourcetypes.Resources
	rollbackPlugins := []plugins.Plugin{}

	return utils.PCR(ctx,
		func(ctx context.Context) error {
			var err error
			nodeCapacity, nodeUsage, _, err = m.getNodeResourceInfo(ctx, nodename, m.plugins, nil, false)
			if err != nil {
				logger.Error(ctx, err, "failed to get node resource")
				return err
			}
			return nil
		},
		func(ctx context.Context) error {
			resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.RemoveNodeResponse, error) {
				resp, err := plugin.RemoveNode(ctx, nodename)
				if err != nil {
					logger.Errorf(ctx, err, "plugin %+v failed to remove node", plugin.Name())
				}
				return resp, err
			})
			if err != nil {
				rollbackPlugins = slices.Collect(maps.Keys(resps))
				logger.Error(ctx, err, "failed to remove node")
				return err
			}
			return nil
		},
		func(ctx context.Context) error {
			err := rollbackNodeResource(ctx, logger, nodename, rollbackPlugins, func(plugin plugins.Plugin) (*plugintypes.SetNodeResourceInfoResponse, error) {
				return plugin.SetNodeResourceInfo(ctx, nodename, nodeCapacity[plugin.Name()], nodeUsage[plugin.Name()])
			})
			if err != nil {
				logger.Error(ctx, err, "failed to rollback")
			}
			return err
		},
		m.config.GlobalTimeout,
	)
}

func (m Manager) GetMostIdleNode(ctx context.Context, nodenames []string) (string, error) {
	logger := log.WithFunc("resource.cobalt.GetMostIdleNode")
	if len(nodenames) == 0 {
		return "", errors.Wrap(types.ErrGetMostIdleNodeFailed, "empty node names")
	}

	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.GetMostIdleNodeResponse, error) {
		resp, err := plugin.GetMostIdleNode(ctx, nodenames)
		if err != nil {
			logger.Errorf(ctx, err, "plugin %+v failed to get the most idle node of %+v", plugin.Name(), nodenames)
		}
		return resp, err
	})
	if err != nil {
		logger.Errorf(ctx, err, "failed to get the most idle node of %+v", nodenames)
		return "", err
	}

	var mostIdleNode *plugintypes.GetMostIdleNodeResponse
	for _, resp := range resps {
		if (mostIdleNode == nil || resp.Priority > mostIdleNode.Priority) && len(resp.Nodename) > 0 {
			mostIdleNode = resp
		}
	}

	if mostIdleNode == nil {
		return "", types.ErrGetMostIdleNodeFailed
	}
	return mostIdleNode.Nodename, nil
}

func (m Manager) GetNodeResourceInfo(ctx context.Context, nodename string, workloads []*types.Workload, fix bool) (resourcetypes.Resources, resourcetypes.Resources, []string, error) {
	ps := m.plugins
	if m.config.ResourcePlugin.Whitelist != nil {
		ps = slices.DeleteFunc(slices.Clone(ps), func(plugin plugins.Plugin) bool {
			return !slices.Contains(m.config.ResourcePlugin.Whitelist, plugin.Name())
		})
	}
	return m.getNodeResourceInfo(ctx, nodename, ps, workloads, fix)
}

func (m Manager) SetNodeResourceUsage(ctx context.Context, nodename string, nodeResource, nodeResourceRequest resourcetypes.Resources, workloadsResource []resourcetypes.Resources, delta, incr bool) (resourcetypes.Resources, resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.cobalt.SetNodeResourceUsage").WithField("node", nodename)
	wrksResource := map[string][]resourcetypes.RawParams{}
	rollbackPlugins := []plugins.Plugin{}
	before := resourcetypes.Resources{}
	after := resourcetypes.Resources{}

	return before, after, utils.PCR(ctx,
		func(_ context.Context) error {
			// [{"cpu-plugin": {"cpu": 1}}, {"cpu-plugin": {"cpu": 1}}] -> {"cpu-plugin": [{"cpu": 1}, {"cpu": 1}]}
			for _, workloadResource := range workloadsResource {
				for plugin, params := range workloadResource {
					if _, ok := wrksResource[plugin]; !ok {
						wrksResource[plugin] = []resourcetypes.RawParams{}
					}
					wrksResource[plugin] = append(wrksResource[plugin], params)
				}
			}
			if nodeResourceRequest == nil {
				nodeResourceRequest = resourcetypes.Resources{}
			}
			return nil
		},
		func(ctx context.Context) error {
			resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.SetNodeResourceUsageResponse, error) {
				return plugin.SetNodeResourceUsage(ctx, nodename, nodeResource[plugin.Name()], nodeResourceRequest[plugin.Name()], wrksResource[plugin.Name()], delta, incr)
			})
			if err != nil {
				for plugin, resp := range resps {
					rollbackPlugins = append(rollbackPlugins, plugin)
					before[plugin.Name()] = resp.Before
					after[plugin.Name()] = resp.After
				}
				logger.Error(ctx, err, "failed to set node resource")
			}
			return err
		},
		func(ctx context.Context) error {
			return rollbackNodeResource(ctx, logger, nodename, rollbackPlugins, func(plugin plugins.Plugin) (*plugintypes.SetNodeResourceUsageResponse, error) {
				return plugin.SetNodeResourceUsage(ctx, nodename, before[plugin.Name()], nil, nil, false, false)
			})
		},
		m.config.GlobalTimeout,
	)
}

// GetNodesDeployCapacity returns the nodes meeting every plugin's requirements, and their total capacity.
// the caller must hold the node locks
func (m Manager) GetNodesDeployCapacity(ctx context.Context, nodenames []string, opts resourcetypes.Resources) (map[string]*plugintypes.NodeDeployCapacity, int, error) {
	logger := log.WithFunc("resource.cobalt.GetNodesDeployCapacity")
	var resp map[string]*plugintypes.NodeDeployCapacity

	resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.GetNodesDeployCapacityResponse, error) {
		resp, err := plugin.GetNodesDeployCapacity(ctx, nodenames, opts[plugin.Name()])
		if err != nil {
			logger.Errorf(ctx, err, "plugin %+v failed to get available nodenames, request %+v", plugin.Name(), opts[plugin.Name()])
		}
		return resp, err
	})
	if err != nil {
		return nil, 0, err
	}

	for _, info := range resps {
		resp = m.mergeCapacity(resp, info.NodeDeployCapacityMap)
	}
	total := 0

	for _, info := range resp {
		info.Rate /= info.Weight
		info.Usage /= info.Weight
		if total == math.MaxInt || info.Capacity == math.MaxInt {
			total = math.MaxInt
		} else {
			total += info.Capacity
		}
	}

	return resp, total, nil
}

// SetNodeResourceCapacity updates node capacity from resource options rather than resource args.
func (m Manager) SetNodeResourceCapacity(ctx context.Context, nodename string, nodeResource, nodeResourceRequest resourcetypes.Resources, delta, incr bool) (resourcetypes.Resources, resourcetypes.Resources, error) {
	logger := log.WithFunc("resource.cobalt.SetNodeResourceCapacity").WithField("node", nodename)

	rollbackPlugins := []plugins.Plugin{}
	before := resourcetypes.Resources{}
	after := resourcetypes.Resources{}

	return before, after, utils.PCR(ctx,
		func(_ context.Context) error {
			if nodeResourceRequest == nil {
				nodeResourceRequest = resourcetypes.Resources{}
			}
			return nil
		},
		func(ctx context.Context) error {
			resps, err := call(ctx, m.plugins, func(plugin plugins.Plugin) (*plugintypes.SetNodeResourceCapacityResponse, error) {
				if nodeResource[plugin.Name()] == nil && nodeResourceRequest[plugin.Name()] == nil {
					return nil, nil
				}
				resp, err := plugin.SetNodeResourceCapacity(ctx, nodename, nodeResource[plugin.Name()], nodeResourceRequest[plugin.Name()], delta, incr)
				if err != nil {
					logger.Errorf(ctx, err, "plugin %+v failed to set node resource capacity", plugin.Name())
				}
				return resp, err
			})
			for plugin, resp := range resps {
				if resp == nil {
					continue
				}
				if err != nil {
					rollbackPlugins = append(rollbackPlugins, plugin)
				}
				before[plugin.Name()] = resp.Before
				after[plugin.Name()] = resp.After
			}
			if err != nil {
				logger.Errorf(ctx, err, "failed to set node resource for node %+v", nodename)
				return err
			}
			return nil
		},
		func(ctx context.Context) error {
			return rollbackNodeResource(ctx, logger, nodename, rollbackPlugins, func(plugin plugins.Plugin) (*plugintypes.SetNodeResourceCapacityResponse, error) {
				return plugin.SetNodeResourceCapacity(ctx, nodename, nil, before[plugin.Name()], false, false)
			})
		},
		m.config.GlobalTimeout,
	)
}

func (m Manager) getNodeResourceInfo(ctx context.Context, nodename string, ps []plugins.Plugin, workloads []*types.Workload, fix bool) (resourcetypes.Resources, resourcetypes.Resources, []string, error) {
	nodeCapacity := resourcetypes.Resources{}
	nodeUsage := resourcetypes.Resources{}
	resourceDiffs := []string{}

	resps, err := call(ctx, ps, func(plugin plugins.Plugin) (*plugintypes.GetNodeResourceInfoResponse, error) {
		var resp *plugintypes.GetNodeResourceInfoResponse
		var err error

		wrks := []plugintypes.WorkloadResource{}

		for _, wrk := range workloads {
			r := wrk.Resources[plugin.Name()]
			wrks = append(wrks, r)
		}

		if fix {
			resp, err = plugin.FixNodeResource(ctx, nodename, wrks)
		} else {
			resp, err = plugin.GetNodeResourceInfo(ctx, nodename, wrks)
		}
		if err != nil {
			log.WithFunc("resource.cobalt.GetNodeResourceInfo").WithField("node", nodename).Errorf(ctx, err, "plugin %+v failed to get node resource", plugin.Name())
		}
		return resp, err
	})
	if err != nil {
		return nil, nil, nil, err
	}

	for plugin, resp := range resps {
		nodeCapacity[plugin.Name()] = resp.Capacity
		nodeUsage[plugin.Name()] = resp.Usage
		resourceDiffs = append(resourceDiffs, resp.Diffs...)
	}

	return nodeCapacity, nodeUsage, resourceDiffs, nil
}

func (m Manager) mergeCapacity(m1, m2 map[string]*plugintypes.NodeDeployCapacity) map[string]*plugintypes.NodeDeployCapacity {
	resp := map[string]*plugintypes.NodeDeployCapacity{}
	if m1 == nil {
		for nodename, info := range m2 {
			resp[nodename] = &plugintypes.NodeDeployCapacity{
				Capacity: info.Capacity,
				Rate:     info.Rate * info.Weight,
				Usage:    info.Usage * info.Weight,
				Weight:   info.Weight,
			}
		}
		return resp
	}

	for nodename, info1 := range m1 {
		if info2, ok := m2[nodename]; ok {
			resp[nodename] = &plugintypes.NodeDeployCapacity{
				Capacity: min(info1.Capacity, info2.Capacity),
				Rate:     info1.Rate + info2.Rate*info2.Weight,
				Usage:    info1.Usage + info2.Usage*info2.Weight,
				Weight:   info1.Weight + info2.Weight,
			}
		}
	}
	return resp
}

// rollbackNodeResource restores every plugin that applied a failed set of the node's resources.
func rollbackNodeResource[T any](ctx context.Context, logger *log.Fields, nodename string, ps []plugins.Plugin, restore func(plugins.Plugin) (T, error)) error {
	_, err := call(ctx, ps, func(plugin plugins.Plugin) (T, error) {
		resp, err := restore(plugin)
		if err != nil {
			logger.Errorf(ctx, err, "node %+v plugin %+v failed to rollback", nodename, plugin.Name())
		}
		return resp, err
	})
	return err
}
