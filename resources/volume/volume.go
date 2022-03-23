package volume

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/resources/volume/models"
	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
)

// VolumePlugin wrapper of volume
type VolumePlugin struct {
	v *models.Volume
}

// NewVolumePlugin .
func NewVolumePlugin(config coretypes.Config) (*VolumePlugin, error) {
	v, err := models.NewVolume(config)
	if err != nil {
		return nil, err
	}
	return &VolumePlugin{v: v}, nil
}

// GetDeployArgs .
func (v *VolumePlugin) GetDeployArgs(ctx context.Context, nodeName string, deployCount int, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetDeployArgsResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}
	engineArgs, resourceArgs, err := v.v.GetDeployArgs(ctx, nodeName, deployCount, workloadResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetDeployArgsResponse{}
	err = resources.ToResp(map[string]interface{}{
		"engine_args":   engineArgs,
		"resource_args": resourceArgs,
	}, resp)
	return resp, err
}

// GetReallocArgs .
func (v *VolumePlugin) GetReallocArgs(ctx context.Context, nodeName string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetReallocArgsResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}
	originWorkloadResourceArgs := &types.WorkloadResourceArgs{}
	if err := originWorkloadResourceArgs.ParseFromRawParams(coretypes.RawParams(originResourceArgs)); err != nil {
		return nil, err
	}

	engineArgs, delta, resourceArgs, err := v.v.GetReallocArgs(ctx, nodeName, originWorkloadResourceArgs, workloadResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetReallocArgsResponse{}
	err = resources.ToResp(map[string]interface{}{
		"engine_args":   engineArgs,
		"delta":         delta,
		"resource_args": resourceArgs,
	}, resp)
	return resp, err
}

// GetRemapArgs .
func (v *VolumePlugin) GetRemapArgs(ctx context.Context, nodeName string, workloadMap map[string]*coretypes.Workload) (*resources.GetRemapArgsResponse, error) {
	workloadResourceArgsMap, err := v.workloadMapToWorkloadResourceArgsMap(workloadMap)
	if err != nil {
		return nil, err
	}

	engineArgs, err := v.v.GetRemapArgs(ctx, nodeName, workloadResourceArgsMap)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetRemapArgsResponse{}
	err = resources.ToResp(map[string]interface{}{
		"engine_args": engineArgs,
	}, resp)
	return resp, err
}

// GetNodesDeployCapacity .
func (v *VolumePlugin) GetNodesDeployCapacity(ctx context.Context, nodeNames []string, resourceOpts coretypes.WorkloadResourceOpts) (*resources.GetNodesDeployCapacityResponse, error) {
	workloadResourceOpts := &types.WorkloadResourceOpts{}
	if err := workloadResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}

	nodesDeployCapacity, total, err := v.v.GetNodesDeployCapacity(ctx, nodeNames, workloadResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetNodesDeployCapacityResponse{}
	err = resources.ToResp(map[string]interface{}{
		"nodes": nodesDeployCapacity,
		"total": total,
	}, resp)
	return resp, err
}

// GetMostIdleNode .
func (v *VolumePlugin) GetMostIdleNode(ctx context.Context, nodeNames []string) (*resources.GetMostIdleNodeResponse, error) {
	nodeName, priority, err := v.v.GetMostIdleNode(ctx, nodeNames)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetMostIdleNodeResponse{}
	err = resources.ToResp(map[string]interface{}{
		"node":     nodeName,
		"priority": priority,
	}, resp)
	return resp, err
}

// GetNodeResourceInfo .
func (v *VolumePlugin) GetNodeResourceInfo(ctx context.Context, nodeName string, workloads []*coretypes.Workload) (*resources.GetNodeResourceInfoResponse, error) {
	return v.getNodeResourceInfo(ctx, nodeName, workloads, false)
}

// FixNodeResource .
func (v *VolumePlugin) FixNodeResource(ctx context.Context, nodeName string, workloads []*coretypes.Workload) (*resources.GetNodeResourceInfoResponse, error) {
	return v.getNodeResourceInfo(ctx, nodeName, workloads, true)
}

// SetNodeResourceUsage .
func (v *VolumePlugin) SetNodeResourceUsage(ctx context.Context, nodeName string, resourceOpts coretypes.NodeResourceOpts, resourceArgs coretypes.NodeResourceArgs, workloadResourceArgs []coretypes.WorkloadResourceArgs, delta bool, incr bool) (*resources.SetNodeResourceUsageResponse, error) {
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

	before, after, err := v.v.SetNodeResourceUsage(ctx, nodeName, nodeResourceOpts, nodeResourceArgs, workloadResourceArgsList, delta, incr)
	if err != nil {
		return nil, err
	}

	resp := &resources.SetNodeResourceUsageResponse{}
	err = resources.ToResp(map[string]interface{}{
		"before": before,
		"after":  after,
	}, resp)
	return resp, err
}

// SetNodeResourceCapacity .
func (v *VolumePlugin) SetNodeResourceCapacity(ctx context.Context, nodeName string, resourceOpts coretypes.NodeResourceOpts, resourceArgs coretypes.NodeResourceArgs, delta bool, incr bool) (*resources.SetNodeResourceCapacityResponse, error) {
	var nodeResourceOpts *types.NodeResourceOpts
	var nodeResourceArgs *types.NodeResourceArgs

	if resourceOpts != nil {
		nodeResourceOpts = &types.NodeResourceOpts{}
		if err := nodeResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
			return nil, err
		}
		if delta {
			nodeResourceOpts.Storage += nodeResourceOpts.Volumes.Total()
		}
	}
	if resourceArgs != nil {
		nodeResourceArgs = &types.NodeResourceArgs{}
		if err := nodeResourceArgs.ParseFromRawParams(coretypes.RawParams(resourceArgs)); err != nil {
			return nil, err
		}
		if delta {
			nodeResourceArgs.Storage += nodeResourceArgs.Volumes.Total()
		}
	}

	before, after, err := v.v.SetNodeResourceCapacity(ctx, nodeName, nodeResourceOpts, nodeResourceArgs, delta, incr)
	if err != nil {
		return nil, err
	}

	resp := &resources.SetNodeResourceCapacityResponse{}
	err = resources.ToResp(map[string]interface{}{
		"before": before,
		"after":  after,
	}, resp)
	return resp, err
}

// SetNodeResourceInfo .
func (v *VolumePlugin) SetNodeResourceInfo(ctx context.Context, nodeName string, resourceCapacity coretypes.NodeResourceArgs, resourceUsage coretypes.NodeResourceArgs) (*resources.SetNodeResourceInfoResponse, error) {
	capacity := &types.NodeResourceArgs{}
	if err := capacity.ParseFromRawParams(coretypes.RawParams(resourceCapacity)); err != nil {
		return nil, err
	}

	usage := &types.NodeResourceArgs{}
	if err := usage.ParseFromRawParams(coretypes.RawParams(resourceUsage)); err != nil {
		return nil, err
	}

	if err := v.v.SetNodeResourceInfo(ctx, nodeName, capacity, usage); err != nil {
		return nil, err
	}
	return &resources.SetNodeResourceInfoResponse{}, nil
}

// AddNode .
func (v *VolumePlugin) AddNode(ctx context.Context, nodeName string, resourceOpts coretypes.NodeResourceOpts, nodeInfo *enginetypes.Info) (*resources.AddNodeResponse, error) {
	nodeResourceOpts := &types.NodeResourceOpts{}
	if err := nodeResourceOpts.ParseFromRawParams(coretypes.RawParams(resourceOpts)); err != nil {
		return nil, err
	}

	// set default value
	if nodeInfo != nil {
		if nodeResourceOpts.Storage == 0 {
			nodeResourceOpts.Storage = nodeInfo.StorageTotal * 8 / 10
		}
	}

	nodeResourceInfo, err := v.v.AddNode(ctx, nodeName, nodeResourceOpts)
	if err != nil {
		return nil, err
	}

	resp := &resources.AddNodeResponse{}
	err = resources.ToResp(map[string]interface{}{
		"capacity": nodeResourceInfo.Capacity,
		"usage":    nodeResourceInfo.Usage,
	}, resp)
	return resp, err
}

// RemoveNode .
func (v *VolumePlugin) RemoveNode(ctx context.Context, nodeName string) (*resources.RemoveNodeResponse, error) {
	if err := v.v.RemoveNode(ctx, nodeName); err != nil {
		return nil, err
	}
	return &resources.RemoveNodeResponse{}, nil
}

// Name .
func (v *VolumePlugin) Name() string {
	return "volume"
}

func (v *VolumePlugin) workloadMapToWorkloadResourceArgsMap(workloadMap map[string]*coretypes.Workload) (*types.WorkloadResourceArgsMap, error) {
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

func (v *VolumePlugin) workloadListToWorkloadResourceArgsMap(workloads []*coretypes.Workload) (*types.WorkloadResourceArgsMap, error) {
	workloadMap := map[string]*coretypes.Workload{}
	for _, workload := range workloads {
		workloadMap[workload.ID] = workload
	}

	return v.workloadMapToWorkloadResourceArgsMap(workloadMap)
}

func (v *VolumePlugin) getNodeResourceInfo(ctx context.Context, nodeName string, workloads []*coretypes.Workload, fix bool) (*resources.GetNodeResourceInfoResponse, error) {
	workloadResourceArgsMap, err := v.workloadListToWorkloadResourceArgsMap(workloads)
	if err != nil {
		return nil, err
	}

	nodeResourceInfo, diffs, err := v.v.GetNodeResourceInfo(ctx, nodeName, workloadResourceArgsMap, fix)
	if err != nil {
		return nil, err
	}

	resp := &resources.GetNodeResourceInfoResponse{}
	err = resources.ToResp(map[string]interface{}{
		"resource_info": nodeResourceInfo,
		"diffs":         diffs,
	}, resp)
	return resp, err
}

// GetMetricsDescription .
func (v *VolumePlugin) GetMetricsDescription(ctx context.Context) (*resources.GetMetricsDescriptionResponse, error) {
	resp := &resources.GetMetricsDescriptionResponse{}
	err := resources.ToResp(v.v.GetMetricsDescription(), resp)
	return resp, err
}

// ResolveNodeResourceInfoToMetrics .
func (v *VolumePlugin) ResolveNodeResourceInfoToMetrics(ctx context.Context, podName string, nodeName string, info *resources.NodeResourceInfo) (*resources.ResolveNodeResourceInfoToMetricsResponse, error) {
	capacity, usage := &types.NodeResourceArgs{}, &types.NodeResourceArgs{}
	if err := capacity.ParseFromRawParams(coretypes.RawParams(info.Capacity)); err != nil {
		return nil, err
	}
	if err := usage.ParseFromRawParams(coretypes.RawParams(info.Usage)); err != nil {
		return nil, err
	}

	metrics := v.v.ResolveNodeResourceInfoToMetrics(podName, nodeName, capacity, usage)
	resp := &resources.ResolveNodeResourceInfoToMetricsResponse{}
	err := resources.ToResp(metrics, resp)
	return resp, err
}
