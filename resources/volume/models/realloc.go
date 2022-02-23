package models

import (
	"context"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/volume/schedule"
	"github.com/projecteru2/core/resources/volume/types"
)

// GetReallocArgs .
func (v *Volume) GetReallocArgs(ctx context.Context, node string, originResourceArgs *types.WorkloadResourceArgs, resourceOpts *types.WorkloadResourceOpts) (*types.EngineArgs, *types.WorkloadResourceArgs, *types.WorkloadResourceArgs, error) {
	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[Realloc] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, nil, err
	}

	finalWorkloadResourceArgs := &types.WorkloadResourceArgs{
		VolumesRequest:    types.MergeVolumeBindings(resourceOpts.VolumesRequest, originResourceArgs.VolumesRequest),
		VolumesLimit:      types.MergeVolumeBindings(resourceOpts.VolumesLimit, originResourceArgs.VolumesLimit),
		VolumePlanRequest: nil,
		VolumePlanLimit:   nil,
		StorageRequest:    resourceOpts.StorageRequest,
		StorageLimit:      resourceOpts.StorageLimit,
	}

	if finalWorkloadResourceArgs.StorageRequest > resourceInfo.Capacity.Storage-resourceInfo.Usage.Storage {
		return nil, nil, nil, types.ErrInsufficientResource
	}

	volumePlan := schedule.GetAffinityPlan(resourceInfo, resourceOpts.VolumesRequest, originResourceArgs.VolumePlanRequest)
	if volumePlan == nil {
		return nil, nil, nil, types.ErrInsufficientResource
	}

	finalWorkloadResourceArgs.VolumePlanRequest = volumePlan
	finalWorkloadResourceArgs.VolumePlanLimit = getVolumePlanLimit(finalWorkloadResourceArgs.VolumesRequest, volumePlan)

	originBindingSet := map[[3]string]struct{}{}
	for binding := range originResourceArgs.VolumePlanLimit {
		originBindingSet[binding.GetMapKey()] = struct{}{}
	}

	engineArgs := &types.EngineArgs{Storage: finalWorkloadResourceArgs.StorageLimit}
	for _, binding := range resourceOpts.VolumesLimit.ApplyPlan(volumePlan) {
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

	return &types.WorkloadResourceArgs{
		VolumePlanRequest: types.VolumePlan{&types.VolumeBinding{
			Source:      "fake-source",
			Destination: "fake-destination",
			Flags:       "fake-flags",
			SizeInBytes: 0,
		}: deltaVolumeMap},
		StorageRequest: finalWorkloadResourceArgs.StorageRequest - originWorkloadResourceArgs.StorageRequest,
	}
}
