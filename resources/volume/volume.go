package volume

import (
	"context"

	"github.com/mitchellh/mapstructure"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/resources/volume/models"
	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

// Plugin wrapper of volume
type Plugin struct {
	v *models.Volume
}

// NewPlugin .
func NewPlugin(config coretypes.Config) (*Plugin, error) {
	v, err := models.NewVolume(config)
	if err != nil {
		return nil, err
	}
	return &Plugin{v: v}, nil
}

// GetDeployArgs .
func (v *Plugin) GetDeployArgs(ctx context.Context, nodename string, deployCount int, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetDeployArgsResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}
	engineArgs, resourceArgs, err := v.v.GetDeployArgs(ctx, nodename, deployCount, workloadResourceOpts)
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
func (v *Plugin) GetReallocArgs(ctx context.Context, nodename string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetReallocArgsResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}
	originWorkloadResourceArgs := &types.WorkloadResourceArgs{}
	if err := originWorkloadResourceArgs.ParseFromRawParams(coretypes.RawParams(originResourceArgs)); err != nil {
		return nil, err
	}

	engineArgs, delta, resourceArgs, err := v.v.GetReallocArgs(ctx, nodename, originWorkloadResourceArgs, workloadResourceOpts)
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
func (v *Plugin) GetRemapArgs(ctx context.Context, nodename string, workloadMap map[string]*coretypes.Workload) (*resources.GetRemapArgsResponse, error) {
	workloadResourceArgsMap, err := v.workloadMapToWorkloadResourceArgsMap(workloadMap)
	if err != nil {
		return nil, err
	}

	engineArgs, err := v.v.GetRemapArgs(ctx, nodename, workloadResourceArgsMap)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetRemapArgsResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_args": engineArgs,
	}, resp)
}

// GetNodesDeployCapacity .
func (v *Plugin) GetNodesDeployCapacity(ctx context.Context, nodeNames []string, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetNodesDeployCapacityResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}

	nodesDeployCapacity, total, err := v.v.GetNodesDeployCapacity(ctx, nodeNames, workloadResourceOpts)
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
func (v *Plugin) GetMostIdleNode(ctx context.Context, nodeNames []string) (*resources.GetMostIdleNodeResponse, error) {
	nodename, priority, err := v.v.GetMostIdleNode(ctx, nodeNames)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetMostIdleNodeResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"nodename": nodename,
		"priority": priority,
	}, resp)
}

// GetNodeResourceInfo .
func (v *Plugin) GetNodeResourceInfo(ctx context.Context, nodename string, workloads []*coretypes.Workload) (*resources.GetNodeResourceInfoResponse, error) {
	return v.getNodeResourceInfo(ctx, nodename, workloads, false)
}

// FixNodeResource .
func (v *Plugin) FixNodeResource(ctx context.Context, nodename string, workloads []*coretypes.Workload) (*resources.GetNodeResourceInfoResponse, error) {
	return v.getNodeResourceInfo(ctx, nodename, workloads, true)
}

// SetNodeResourceUsage .
func (v *Plugin) SetNodeResourceUsage(ctx context.Context, nodename string, resourceOpts coretypes.NodeResourceOpts, resourceArgs coretypes.NodeResourceArgs, workloadResourceArgs []coretypes.WorkloadResourceArgs, delta bool, incr bool) (*resources.SetNodeResourceUsageResponse, error) {
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

	before, after, err := v.v.SetNodeResourceUsage(ctx, nodename, nodeResourceOpts, nodeResourceArgs, workloadResourceArgsList, delta, incr)
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
func (v *Plugin) SetNodeResourceCapacity(ctx context.Context, nodename string, resourceOpts coretypes.NodeResourceOpts, resourceArgs coretypes.NodeResourceArgs, delta bool, incr bool) (*resources.SetNodeResourceCapacityResponse, error) {
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

	before, after, err := v.v.SetNodeResourceCapacity(ctx, nodename, nodeResourceOpts, nodeResourceArgs, delta, incr)
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
func (v *Plugin) SetNodeResourceInfo(ctx context.Context, nodename string, resourceCapacity coretypes.NodeResourceArgs, resourceUsage coretypes.NodeResourceArgs) (*resources.SetNodeResourceInfoResponse, error) {
	capacity := &types.NodeResourceArgs{}
	if err := capacity.ParseFromRawParams(coretypes.RawParams(resourceCapacity)); err != nil {
		return nil, err
	}

	usage := &types.NodeResourceArgs{}
	if err := usage.ParseFromRawParams(coretypes.RawParams(resourceUsage)); err != nil {
		return nil, err
	}

	if err := v.v.SetNodeResourceInfo(ctx, nodename, capacity, usage); err != nil {
		return nil, err
	}
	return &resources.SetNodeResourceInfoResponse{}, nil
}

// AddNode .
func (v *Plugin) AddNode(ctx context.Context, nodename string, resourceOpts coretypes.NodeResourceOpts, nodeInfo *enginetypes.Info) (*resources.AddNodeResponse, error) {
	nodeResourceOpts := &types.NodeResourceOpts{}
	if err := nodeResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}

	// set default value
	if nodeInfo != nil && nodeResourceOpts.Storage == 0 {
		nodeResourceOpts.Storage = nodeInfo.StorageTotal * 8 / 10
	}

	nodeResourceInfo, err := v.v.AddNode(ctx, nodename, nodeResourceOpts)
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
func (v *Plugin) RemoveNode(ctx context.Context, nodename string) (*resources.RemoveNodeResponse, error) {
	if err := v.v.RemoveNode(ctx, nodename); err != nil {
		return nil, err
	}
	return &resources.RemoveNodeResponse{}, nil
}

// Name .
func (v *Plugin) Name() string {
	return "volume"
}

func (v *Plugin) workloadMapToWorkloadResourceArgsMap(workloadMap map[string]*coretypes.Workload) (*types.WorkloadResourceArgsMap, error) {
	workloadResourceArgsMap := types.WorkloadResourceArgsMap{}
	for workloadID, workload := range workloadMap {
		workloadResourceArgs := &types.WorkloadResourceArgs{}
		if err := workloadResourceArgs.ParseFromRawParams(coretypes.RawParams(workload.ResourceArgs[v.Name()])); err != nil {
			return nil, err
		}
		workloadResourceArgsMap[workloadID] = workloadResourceArgs
	}

	return &workloadResourceArgsMap, nil
}

func (v *Plugin) workloadListToWorkloadResourceArgsMap(workloads []*coretypes.Workload) (*types.WorkloadResourceArgsMap, error) {
	workloadMap := map[string]*coretypes.Workload{}
	for _, workload := range workloads {
		workloadMap[workload.ID] = workload
	}

	return v.workloadMapToWorkloadResourceArgsMap(workloadMap)
}

func (v *Plugin) getNodeResourceInfo(ctx context.Context, nodename string, workloads []*coretypes.Workload, fix bool) (*resources.GetNodeResourceInfoResponse, error) {
	workloadResourceArgsMap, err := v.workloadListToWorkloadResourceArgsMap(workloads)
	if err != nil {
		return nil, err
	}

	nodeResourceInfo, diffs, err := v.v.GetNodeResourceInfo(ctx, nodename, workloadResourceArgsMap, fix)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetNodeResourceInfoResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"resource_info": nodeResourceInfo,
		"diffs":         diffs,
	}, resp)
}

// GetMetricsDescription .
func (v *Plugin) GetMetricsDescription(ctx context.Context) (*resources.GetMetricsDescriptionResponse, error) {
	resp := &resources.GetMetricsDescriptionResponse{}
	return resp, mapstructure.Decode(v.v.GetMetricsDescription(), resp)
}

// GetNodeMetrics .
func (v *Plugin) GetNodeMetrics(ctx context.Context, podname string, nodename string, info *resources.NodeResourceInfo) (*resources.GetNodeMetricsResponse, error) {
	capacity, usage := &types.NodeResourceArgs{}, &types.NodeResourceArgs{}
	if err := capacity.ParseFromRawParams(coretypes.RawParams(info.Capacity)); err != nil {
		return nil, err
	}
	if err := usage.ParseFromRawParams(coretypes.RawParams(info.Usage)); err != nil {
		return nil, err
	}

	metrics := v.v.GetNodeMetrics(podname, nodename, capacity, usage)
	resp := &resources.GetNodeMetricsResponse{}
	return resp, mapstructure.Decode(metrics, resp)
}
