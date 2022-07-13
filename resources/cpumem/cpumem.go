package cpumem

import (
	"context"
	"strconv"

	"github.com/mitchellh/mapstructure"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/resources/cpumem/models"
	"github.com/projecteru2/core/resources/cpumem/types"
	coretypes "github.com/projecteru2/core/types"
)

// Plugin wrapper of CPUMem
type Plugin struct {
	c *models.CPUMem
}

// NewPlugin creates a new Plugin
func NewPlugin(config coretypes.Config) (*Plugin, error) {
	c, err := models.NewCPUMem(config)
	if err != nil {
		return nil, err
	}
	return &Plugin{c: c}, nil
}

// GetDeployArgs .
func (c *Plugin) GetDeployArgs(ctx context.Context, nodename string, deployCount int, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetDeployArgsResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}
	engineArgs, resourceArgs, err := c.c.GetDeployArgs(ctx, nodename, deployCount, workloadResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetDeployArgsResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_args":   engineArgs,
		"resource_args": resourceArgs,
	}, resp)
}

// GetReallocArgs .
func (c *Plugin) GetReallocArgs(ctx context.Context, nodename string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetReallocArgsResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}
	originWorkloadResourceArgs := &types.WorkloadResourceArgs{}
	if err := originWorkloadResourceArgs.ParseFromRawParams(coretypes.RawParams(originResourceArgs)); err != nil {
		return nil, err
	}

	engineArgs, delta, resourceArgs, err := c.c.GetReallocArgs(ctx, nodename, originWorkloadResourceArgs, workloadResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetReallocArgsResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_args":   engineArgs,
		"delta":         delta,
		"resource_args": resourceArgs,
	}, resp)
}

// GetRemapArgs .
func (c *Plugin) GetRemapArgs(ctx context.Context, nodename string, workloadMap map[string]*coretypes.Workload) (*resources.GetRemapArgsResponse, error) {
	workloadResourceArgsMap, err := c.workloadMapToWorkloadResourceArgsMap(workloadMap)
	if err != nil {
		return nil, err
	}

	engineArgs, err := c.c.GetRemapArgs(ctx, nodename, workloadResourceArgsMap)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetRemapArgsResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_args": engineArgs,
	}, resp)
}

// GetNodesDeployCapacity .
func (c *Plugin) GetNodesDeployCapacity(ctx context.Context, nodeNames []string, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetNodesDeployCapacityResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}

	nodesDeployCapacity, total, err := c.c.GetNodesDeployCapacity(ctx, nodeNames, workloadResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetNodesDeployCapacityResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"nodes": nodesDeployCapacity,
		"total": total,
	}, resp)
}

// GetMostIdleNode .
func (c *Plugin) GetMostIdleNode(ctx context.Context, nodeNames []string) (*resources.GetMostIdleNodeResponse, error) {
	nodename, priority, err := c.c.GetMostIdleNode(ctx, nodeNames)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetMostIdleNodeResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"node":     nodename,
		"priority": priority,
	}, resp)
}

// GetNodeResourceInfo .
func (c *Plugin) GetNodeResourceInfo(ctx context.Context, nodename string, workloads []*coretypes.Workload, fix bool) (*resources.GetNodeResourceInfoResponse, error) {
	workloadResourceArgsMap, err := c.workloadListToWorkloadResourceArgsMap(workloads)
	if err != nil {
		return nil, err
	}

	nodeResourceInfo, diffs, err := c.c.GetNodeResourceInfo(ctx, nodename, workloadResourceArgsMap, fix)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetNodeResourceInfoResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"resource_info": nodeResourceInfo,
		"diffs":         diffs,
	}, resp)
}

// SetNodeResourceUsage .
func (c *Plugin) SetNodeResourceUsage(ctx context.Context, nodename string, resourceOpts coretypes.NodeResourceOpts, resourceArgs coretypes.NodeResourceArgs, workloadResourceArgs []coretypes.WorkloadResourceArgs, delta bool, incr bool) (*resources.SetNodeResourceUsageResponse, error) {
	var nodeResourceOpts *types.NodeResourceOpts
	var nodeResourceArgs *types.NodeResourceArgs
	var workloadResourceArgsList []*types.WorkloadResourceArgs

	if resourceOpts != nil {
		nodeResourceOpts = &types.NodeResourceOpts{}
		if err := nodeResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
			return nil, err
		}
	}

	if resourceArgs != nil {
		nodeResourceArgs = &types.NodeResourceArgs{}
		if err := nodeResourceArgs.ParseFromRawParams(coretypes.RawParams(resourceArgs)); err != nil {
			return nil, err
		}
	}

	if workloadResourceArgs != nil {
		workloadResourceArgsList = make([]*types.WorkloadResourceArgs, len(workloadResourceArgs))
		for i, workloadResourceArg := range workloadResourceArgs {
			workloadResourceArgsList[i] = &types.WorkloadResourceArgs{}
			if err := workloadResourceArgsList[i].ParseFromRawParams(coretypes.RawParams(workloadResourceArg)); err != nil {
				return nil, err
			}
		}
	}

	before, after, err := c.c.SetNodeResourceUsage(ctx, nodename, nodeResourceOpts, nodeResourceArgs, workloadResourceArgsList, delta, incr)
	if err != nil {
		return nil, err
	}

	resp := &resources.SetNodeResourceUsageResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"before": before,
		"after":  after,
	}, resp)
}

// SetNodeResourceCapacity .
func (c *Plugin) SetNodeResourceCapacity(ctx context.Context, nodename string, resourceOpts coretypes.NodeResourceOpts, resourceArgs coretypes.NodeResourceArgs, delta bool, incr bool) (*resources.SetNodeResourceCapacityResponse, error) {
	var nodeResourceOpts *types.NodeResourceOpts
	var nodeResourceArgs *types.NodeResourceArgs

	if resourceOpts != nil {
		nodeResourceOpts = &types.NodeResourceOpts{}
		if err := nodeResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
			return nil, err
		}
	}
	if resourceArgs != nil {
		nodeResourceArgs = &types.NodeResourceArgs{}
		if err := nodeResourceArgs.ParseFromRawParams(coretypes.RawParams(resourceArgs)); err != nil {
			return nil, err
		}
	}

	before, after, err := c.c.SetNodeResourceCapacity(ctx, nodename, nodeResourceOpts, nodeResourceArgs, delta, incr)
	if err != nil {
		return nil, err
	}

	resp := &resources.SetNodeResourceCapacityResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"before": before,
		"after":  after,
	}, resp)
}

// SetNodeResourceInfo .
func (c *Plugin) SetNodeResourceInfo(ctx context.Context, nodename string, resourceCapacity coretypes.NodeResourceArgs, resourceUsage coretypes.NodeResourceArgs) (*resources.SetNodeResourceInfoResponse, error) {
	capacity := &types.NodeResourceArgs{}
	if err := capacity.ParseFromRawParams(coretypes.RawParams(resourceCapacity)); err != nil {
		return nil, err
	}

	usage := &types.NodeResourceArgs{}
	if err := usage.ParseFromRawParams(coretypes.RawParams(resourceUsage)); err != nil {
		return nil, err
	}

	if err := c.c.SetNodeResourceInfo(ctx, nodename, capacity, usage); err != nil {
		return nil, err
	}
	return &resources.SetNodeResourceInfoResponse{}, nil
}

// AddNode .
func (c *Plugin) AddNode(ctx context.Context, nodename string, resourceOpts coretypes.NodeResourceOpts, nodeInfo *enginetypes.Info) (*resources.AddNodeResponse, error) {
	nodeResourceOpts := &types.NodeResourceOpts{}
	if err := nodeResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}

	// reset by node info default value
	if nodeInfo != nil {
		if len(nodeResourceOpts.CPUMap) == 0 {
			nodeResourceOpts.CPUMap = types.CPUMap{}
			for i := 0; i < nodeInfo.NCPU; i++ {
				nodeResourceOpts.CPUMap[strconv.Itoa(i)] = c.c.Config.Scheduler.ShareBase
			}
		}

		if nodeResourceOpts.Memory == 0 {
			nodeResourceOpts.Memory = nodeInfo.MemTotal * 8 / 10 // use 80% of real memory
		}
	}

	nodeResourceInfo, err := c.c.AddNode(ctx, nodename, nodeResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.AddNodeResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"capacity": nodeResourceInfo.Capacity,
		"usage":    nodeResourceInfo.Usage,
	}, resp)
}

// RemoveNode .
func (c *Plugin) RemoveNode(ctx context.Context, nodename string) (*resources.RemoveNodeResponse, error) {
	if err := c.c.RemoveNode(ctx, nodename); err != nil {
		return nil, err
	}
	return &resources.RemoveNodeResponse{}, nil
}

// Name .
func (c *Plugin) Name() string {
	return "cpumem"
}

func (c *Plugin) workloadMapToWorkloadResourceArgsMap(workloadMap map[string]*coretypes.Workload) (*types.WorkloadResourceArgsMap, error) {
	workloadResourceArgsMap := types.WorkloadResourceArgsMap{}
	for workloadID, workload := range workloadMap {
		workloadResourceArgs := &types.WorkloadResourceArgs{}
		if err := workloadResourceArgs.ParseFromRawParams(coretypes.RawParams(workload.ResourceArgs[c.Name()])); err != nil {
			return nil, err
		}
		workloadResourceArgsMap[workloadID] = workloadResourceArgs
	}

	return &workloadResourceArgsMap, nil
}

func (c *Plugin) workloadListToWorkloadResourceArgsMap(workloads []*coretypes.Workload) (*types.WorkloadResourceArgsMap, error) {
	workloadMap := map[string]*coretypes.Workload{}
	for _, workload := range workloads {
		workloadMap[workload.ID] = workload
	}

	return c.workloadMapToWorkloadResourceArgsMap(workloadMap)
}

// GetMetricsDescription .
func (c *Plugin) GetMetricsDescription(ctx context.Context) (*resources.GetMetricsDescriptionResponse, error) {
	resp := &resources.GetMetricsDescriptionResponse{}
	return resp, mapstructure.Decode(c.c.GetMetricsDescription(), resp)
}

// ConvertNodeResourceInfoToMetrics .
func (c *Plugin) ConvertNodeResourceInfoToMetrics(ctx context.Context, podname string, nodename string, info *resources.NodeResourceInfo) (*resources.ConvertNodeResourceInfoToMetricsResponse, error) {
	capacity, usage := &types.NodeResourceArgs{}, &types.NodeResourceArgs{}
	if err := capacity.ParseFromRawParams(coretypes.RawParams(info.Capacity)); err != nil {
		return nil, err
	}
	if err := usage.ParseFromRawParams(coretypes.RawParams(info.Usage)); err != nil {
		return nil, err
	}

	metrics := c.c.ConvertNodeResourceInfoToMetrics(podname, nodename, capacity, usage)
	resp := &resources.ConvertNodeResourceInfoToMetricsResponse{}
	return resp, mapstructure.Decode(metrics, resp)
}
