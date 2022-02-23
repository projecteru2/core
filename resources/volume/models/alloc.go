package models

import (
	"context"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/volume/schedule"
	"github.com/projecteru2/core/resources/volume/types"
)

// GetDeployArgs .
func (v *Volume) GetDeployArgs(ctx context.Context, node string, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	if err := opts.Validate(); err != nil {
		logrus.Errorf("[Alloc] invalid resource opts %+v, err: %v", opts, err)
		return nil, nil, err
	}

	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[Alloc] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, err
	}

	return v.doAlloc(resourceInfo, deployCount, opts)
}

func maxInt64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func getVolumePlanLimit(bindings types.VolumeBindings, volumePlan types.VolumePlan) types.VolumePlan {
	volumeBindingToVolumeMap := map[[3]string]types.VolumeMap{}
	for binding, volumeMap := range volumePlan {
		volumeBindingToVolumeMap[binding.GetMapKey()] = volumeMap
	}

	volumePlanLimit := types.VolumePlan{}

	for _, binding := range bindings {
		if volumeMap, ok := volumeBindingToVolumeMap[binding.GetMapKey()]; ok {
			volumePlanLimit[binding] = types.VolumeMap{volumeMap.GetDevice(): maxInt64(binding.SizeInBytes, volumeMap.GetSize())}
		}
	}
	return volumePlanLimit
}

func (v *Volume) doAlloc(resourceInfo *types.NodeResourceInfo, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	// check if storage is enough
	if opts.StorageRequest > 0 {
		storageCapacity := int((resourceInfo.Capacity.Storage - resourceInfo.Usage.Storage) / opts.StorageRequest)
		if storageCapacity < deployCount {
			return nil, nil, errors.Wrapf(types.ErrInsufficientResource, "not enough storage, request: %v, available: %v", opts.StorageRequest, storageCapacity)
		}
	}

	resEngineArgs := []*types.EngineArgs{}
	resResourceArgs := []*types.WorkloadResourceArgs{}

	// if volume is not required
	if len(opts.VolumesRequest) == 0 {
		for i := 0; i < deployCount; i++ {
			resEngineArgs = append(resEngineArgs, &types.EngineArgs{
				Storage: opts.StorageLimit,
			})
			resResourceArgs = append(resResourceArgs, &types.WorkloadResourceArgs{
				StorageRequest: opts.StorageRequest,
				StorageLimit:   opts.StorageLimit,
			})
		}
		return resEngineArgs, resResourceArgs, nil
	}

	volumePlans := schedule.GetVolumePlans(resourceInfo, opts.VolumesRequest, v.config.Scheduler.MaxDeployCount)
	if len(volumePlans) < deployCount {
		return nil, nil, errors.Wrapf(types.ErrInsufficientResource, "not enough volume plan, need %v, available %v", deployCount, len(volumePlans))
	}

	volumePlans = volumePlans[:deployCount]
	volumeSizeLimitMap := map[*types.VolumeBinding]int64{}
	for _, binding := range opts.VolumesLimit {
		volumeSizeLimitMap[binding] = binding.SizeInBytes
	}

	for _, volumePlan := range volumePlans {
		engineArgs := &types.EngineArgs{Storage: opts.StorageLimit}
		for _, binding := range opts.VolumesLimit.ApplyPlan(volumePlan) {
			engineArgs.Volumes = append(engineArgs.Volumes, binding.ToString(true))
		}

		resourceArgs := &types.WorkloadResourceArgs{
			VolumesRequest:    opts.VolumesRequest,
			VolumesLimit:      opts.VolumesLimit,
			VolumePlanRequest: volumePlan,
			VolumePlanLimit:   getVolumePlanLimit(opts.VolumesLimit, volumePlan),
			StorageRequest:    opts.StorageRequest,
			StorageLimit:      opts.StorageLimit,
		}

		resEngineArgs = append(resEngineArgs, engineArgs)
		resResourceArgs = append(resResourceArgs, resourceArgs)
	}

	return resEngineArgs, resResourceArgs, nil
}
