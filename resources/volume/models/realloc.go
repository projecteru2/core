package models

import (
	"context"

	"github.com/sanity-io/litter"
	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/volume/schedule"
	"github.com/projecteru2/core/resources/volume/types"
	"github.com/projecteru2/core/utils"
)

// GetReallocArgs .
func (v *Volume) GetReallocArgs(ctx context.Context, node string, originResourceArgs *types.WorkloadResourceArgs, resourceOpts *types.WorkloadResourceOpts) (*types.EngineArgs, *types.WorkloadResourceArgs, *types.WorkloadResourceArgs, error) {
	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[Realloc] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, nil, err
	}

	// check if volume rescheduling is needed
	needVolumeReschedule := utils.Any(resourceOpts.VolumesRequest, func(volume *types.VolumeBinding) bool { return volume.RequireSchedule() || volume.RequireIOPS() })

	resourceOpts = &types.WorkloadResourceOpts{
		VolumesRequest: types.MergeVolumeBindings(resourceOpts.VolumesRequest, originResourceArgs.VolumesRequest),
		VolumesLimit:   types.MergeVolumeBindings(resourceOpts.VolumesLimit, originResourceArgs.VolumesLimit),
		StorageRequest: resourceOpts.StorageRequest + originResourceArgs.StorageRequest + resourceOpts.VolumesRequest.TotalSize(),
		StorageLimit:   resourceOpts.StorageLimit + originResourceArgs.StorageLimit + resourceOpts.VolumesLimit.TotalSize(),
	}
	resourceOpts.SkipAddStorage()

	if err := resourceOpts.Validate(); err != nil {
		logrus.Errorf("[Realloc] invalid resource opts %v, err: %v", litter.Sdump(resourceOpts), err)
		return nil, nil, nil, err
	}

	finalWorkloadResourceArgs := &types.WorkloadResourceArgs{
		VolumesRequest:    resourceOpts.VolumesRequest,
		VolumesLimit:      resourceOpts.VolumesLimit,
		VolumePlanRequest: nil,
		VolumePlanLimit:   nil,
		StorageRequest:    resourceOpts.StorageRequest,
		StorageLimit:      resourceOpts.StorageLimit,
	}

	if finalWorkloadResourceArgs.StorageRequest-originResourceArgs.StorageRequest > resourceInfo.Capacity.Storage-resourceInfo.Usage.Storage {
		return nil, nil, nil, types.ErrInsufficientResource
	}

	var volumePlan types.VolumePlan
	var diskPlan types.Disks
	if needVolumeReschedule {
		volumePlan, diskPlan, err = schedule.GetAffinityPlan(resourceInfo, resourceOpts.VolumesRequest, originResourceArgs.VolumePlanRequest, originResourceArgs.VolumesRequest)
		if err != nil {
			return nil, nil, nil, types.ErrInsufficientResource
		}
	} else {
		volumePlan = originResourceArgs.VolumePlanRequest
	}

	finalWorkloadResourceArgs.VolumePlanRequest = volumePlan
	finalWorkloadResourceArgs.DisksRequest = diskPlan
	finalWorkloadResourceArgs.VolumePlanLimit = getVolumePlanLimit(finalWorkloadResourceArgs.VolumesRequest, finalWorkloadResourceArgs.VolumesLimit, volumePlan)
	finalWorkloadResourceArgs.DisksLimit = getDisksLimit(resourceOpts.VolumesLimit, finalWorkloadResourceArgs.VolumePlanLimit, resourceInfo.Capacity.Disks)

	// compute engine args
	originBindingSet := map[[3]string]struct{}{}
	for _, binding := range originResourceArgs.VolumesLimit.ApplyPlan(originResourceArgs.VolumePlanLimit) {
		originBindingSet[binding.GetMapKey()] = struct{}{}
	}

	engineArgs := &types.EngineArgs{Storage: finalWorkloadResourceArgs.StorageLimit, IOPSOptions: v.toIOPSOptions(finalWorkloadResourceArgs.DisksLimit)}
	newBindings := resourceOpts.VolumesLimit.ApplyPlan(volumePlan)
	if len(newBindings) != len(originBindingSet) {
		engineArgs.VolumeChanged = true
	}
	for _, binding := range newBindings {
		engineArgs.Volumes = append(engineArgs.Volumes, binding.ToString(true))
		if _, ok := originBindingSet[binding.GetMapKey()]; !ok {
			engineArgs.VolumeChanged = true
		}
	}

	deltaWorkloadResourceArgs := getDeltaWorkloadResourceArgs(originResourceArgs, finalWorkloadResourceArgs)
	return engineArgs, deltaWorkloadResourceArgs, finalWorkloadResourceArgs, nil
}

func getDeltaWorkloadResourceArgs(originWorkloadResourceArgs, finalWorkloadResourceArgs *types.WorkloadResourceArgs) *types.WorkloadResourceArgs {
	deltaVolumeMap := types.VolumeMap{}
	for _, volumeMap := range finalWorkloadResourceArgs.VolumePlanRequest {
		deltaVolumeMap.Add(volumeMap)
	}
	for _, volumeMap := range originWorkloadResourceArgs.VolumePlanRequest {
		deltaVolumeMap.Sub(volumeMap)
	}

	deltaDisks := finalWorkloadResourceArgs.DisksRequest.DeepCopy()
	deltaDisks.Sub(originWorkloadResourceArgs.DisksRequest)

	return &types.WorkloadResourceArgs{
		VolumePlanRequest: types.VolumePlan{&types.VolumeBinding{
			Source:      "fake-source",
			Destination: "fake-destination",
			Flags:       "fake-flags",
			SizeInBytes: 0,
		}: deltaVolumeMap},
		StorageRequest: finalWorkloadResourceArgs.StorageRequest - originWorkloadResourceArgs.StorageRequest,
		DisksRequest:   deltaDisks,
	}
}
