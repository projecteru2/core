package storage

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource3/plugins/storage/schedule"
	storagetypes "github.com/projecteru2/core/resource3/plugins/storage/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/sanity-io/litter"
)

func (p Plugin) CalculateDeploy(ctx context.Context, nodename string, deployCount int, resourceRequest *plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateDeployResponse, error) {
	logger := log.WithFunc("resource.storage.CalculateDeploy").WithField("node", nodename)
	req := &storagetypes.WorkloadResourceRequest{}
	if err := req.Parse(resourceRequest); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		logger.Errorf(ctx, err, "invalid resource opts %+v", req)
		return nil, err
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err, "failed to get resource info of node")
		return nil, err
	}

	enginesParams, workloadsResource, err := p.doAlloc(nodeResourceInfo, deployCount, req)
	if err != nil {
		return nil, err
	}

	resp := &plugintypes.CalculateDeployResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engines_params":     enginesParams,
		"workloads_resource": workloadsResource,
	}, resp)
}

func (p Plugin) CalculateRealloc(ctx context.Context, nodename string, resource *plugintypes.WorkloadResource, resourceRequest *plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateReallocResponse, error) {
	logger := log.WithFunc("resource.storage.CalculateRealloc").WithField("node", nodename)
	req := &storagetypes.WorkloadResourceRequest{}
	if err := req.Parse(resourceRequest); err != nil {
		return nil, err
	}
	originResource := &storagetypes.WorkloadResource{}
	if err := originResource.Parse(resource); err != nil {
		return nil, err
	}

	resourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err, "failed to get resource info of node")
		return nil, err
	}

	// check if volume rescheduling is needed
	needVolumeReschedule := utils.Any(req.VolumesRequest, func(volume *storagetypes.VolumeBinding) bool { return volume.RequireSchedule() || volume.RequireIOPS() })

	req = &storagetypes.WorkloadResourceRequest{
		VolumesRequest: storagetypes.MergeVolumeBindings(req.VolumesRequest, originResource.VolumesRequest),
		VolumesLimit:   storagetypes.MergeVolumeBindings(req.VolumesLimit, originResource.VolumesLimit),
		StorageRequest: req.StorageRequest + originResource.StorageRequest + req.VolumesRequest.TotalSize(),
		StorageLimit:   req.StorageLimit + originResource.StorageLimit + req.VolumesLimit.TotalSize(),
	}
	req.SkipAddStorage()

	if err := req.Validate(); err != nil {
		logger.Errorf(ctx, err, "invalid resource opts %+v", litter.Sdump(req))
		return nil, err
	}

	targetWorkloadResource := &storagetypes.WorkloadResource{
		VolumesRequest:    req.VolumesRequest,
		VolumesLimit:      req.VolumesLimit,
		VolumePlanRequest: nil,
		VolumePlanLimit:   nil,
		StorageRequest:    req.StorageRequest,
		StorageLimit:      req.StorageLimit,
	}

	if targetWorkloadResource.StorageRequest-originResource.StorageRequest > resourceInfo.Capacity.Storage-resourceInfo.Usage.Storage {
		return nil, coretypes.ErrInsufficientResource
	}

	var volumePlan storagetypes.VolumePlan
	var diskPlan storagetypes.Disks

	if needVolumeReschedule {
		if volumePlan, diskPlan, err = schedule.GetAffinityPlan(resourceInfo, req.VolumesRequest, originResource.VolumePlanRequest, originResource.VolumesRequest); err != nil {
			return nil, coretypes.ErrInsufficientResource
		}
	} else {
		volumePlan = originResource.VolumePlanRequest
	}

	targetWorkloadResource.VolumePlanRequest = volumePlan
	targetWorkloadResource.DisksRequest = diskPlan
	targetWorkloadResource.VolumePlanLimit = getVolumePlanLimit(targetWorkloadResource.VolumesRequest, targetWorkloadResource.VolumesLimit, volumePlan)
	targetWorkloadResource.DisksLimit = getDisksLimit(req.VolumesLimit, targetWorkloadResource.VolumePlanLimit, resourceInfo.Capacity.Disks)

	// compute engine args
	originBindingSet := map[[3]string]struct{}{}
	for _, binding := range originResource.VolumesLimit.ApplyPlan(originResource.VolumePlanLimit) {
		originBindingSet[binding.GetMapKey()] = struct{}{}
	}

	engineParams := &storagetypes.EngineParams{Storage: targetWorkloadResource.StorageLimit, IOPSOptions: p.toIOPSOptions(targetWorkloadResource.DisksLimit)}
	newBindings := req.VolumesLimit.ApplyPlan(volumePlan)
	if len(newBindings) != len(originBindingSet) {
		engineParams.VolumeChanged = true
	}
	for _, binding := range newBindings {
		engineParams.Volumes = append(engineParams.Volumes, binding.ToString(true))
		if _, ok := originBindingSet[binding.GetMapKey()]; !ok {
			engineParams.VolumeChanged = true
		}
	}

	deltaWorkloadResource := getDeltaWorkloadResourceArgs(originResource, targetWorkloadResource)

	resp := &plugintypes.CalculateReallocResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_params":     engineParams,
		"delta_resource":    deltaWorkloadResource,
		"workload_resource": targetWorkloadResource,
	}, resp)
}

func (p Plugin) CalculateRemap(ctx context.Context, nodename string, workloadsResource map[string]*plugintypes.WorkloadResource) (*plugintypes.CalculateRemapResponse, error) {
	// NO NEED REMAP VOLUME
	resp := &plugintypes.CalculateRemapResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_params_map": nil,
	}, resp)
}

func (p Plugin) doAlloc(resourceInfo *storagetypes.NodeResourceInfo, deployCount int, req *storagetypes.WorkloadResourceRequest) ([]*storagetypes.EngineParams, []*storagetypes.WorkloadResource, error) {
	// check if storage is enough
	if req.StorageRequest > 0 {
		storageCapacity := int((resourceInfo.Capacity.Storage - resourceInfo.Usage.Storage) / req.StorageRequest)
		if storageCapacity < deployCount {
			return nil, nil, errors.Wrapf(coretypes.ErrInsufficientResource, "not enough storage, request: %+v, available: %+v", req.StorageRequest, storageCapacity)
		}
	}

	var enginesParams []*storagetypes.EngineParams
	var workloadsResource []*storagetypes.WorkloadResource

	var volumePlans []storagetypes.VolumePlan
	var diskPlans []storagetypes.Disks

	// if volume scheduling is not required
	if !utils.Any(req.VolumesRequest, func(b *storagetypes.VolumeBinding) bool { return b.RequireSchedule() || b.RequireIOPS() }) {
		for i := 0; i < deployCount; i++ {
			volumePlans = append(volumePlans, storagetypes.VolumePlan{})
			diskPlans = append(diskPlans, storagetypes.Disks{})
		}
	} else {
		volumePlans, diskPlans = schedule.GetVolumePlans(resourceInfo, req.VolumesRequest, p.config.Scheduler.MaxDeployCount)
		if len(volumePlans) < deployCount {
			return nil, nil, errors.Wrapf(coretypes.ErrInsufficientResource, "not enough volume plan, need %+v, available %+v", deployCount, len(volumePlans))
		}
		volumePlans = volumePlans[:deployCount]
		diskPlans = diskPlans[:deployCount]
	}

	volumeSizeLimitMap := map[*storagetypes.VolumeBinding]int64{}
	for _, binding := range req.VolumesLimit {
		volumeSizeLimitMap[binding] = binding.SizeInBytes
	}

	for index, volumePlan := range volumePlans {
		engineParam := &storagetypes.EngineParams{Storage: req.StorageLimit}
		for _, binding := range req.VolumesLimit.ApplyPlan(volumePlan) {
			engineParam.Volumes = append(engineParam.Volumes, binding.ToString(true))
		}

		volumePlanLimit := getVolumePlanLimit(req.VolumesLimit, req.VolumesLimit, volumePlan)
		disksLimit := getDisksLimit(req.VolumesLimit, volumePlanLimit, resourceInfo.Capacity.Disks)

		engineParam.IOPSOptions = p.toIOPSOptions(disksLimit)

		workloadResource := &storagetypes.WorkloadResource{
			VolumesRequest:    req.VolumesRequest,
			VolumesLimit:      req.VolumesLimit,
			VolumePlanRequest: volumePlan,
			VolumePlanLimit:   volumePlanLimit,
			StorageRequest:    req.StorageRequest,
			StorageLimit:      req.StorageLimit,
			DisksRequest:      diskPlans[index],
			DisksLimit:        disksLimit,
		}

		enginesParams = append(enginesParams, engineParam)
		workloadsResource = append(workloadsResource, workloadResource)
	}

	return enginesParams, workloadsResource, nil
}
