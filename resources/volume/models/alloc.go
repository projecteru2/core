package models

import (
	"context"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/volume/schedule"
	"github.com/projecteru2/core/resources/volume/types"
	"github.com/projecteru2/core/utils"
)

// GetDeployArgs .
func (v *Volume) GetDeployArgs(ctx context.Context, node string, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	if err := opts.Validate(); err != nil {
		log.Errorf(ctx, err, "[Alloc] invalid resource opts %+v", opts)
		return nil, nil, err
	}

	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		log.Errorf(ctx, err, "[Alloc] failed to get resource info of node %+v", node)
		return nil, nil, err
	}

	return v.doAlloc(resourceInfo, deployCount, opts)
}

func getVolumePlanLimit(volumeRequest types.VolumeBindings, volumeLimit types.VolumeBindings, volumePlan types.VolumePlan) types.VolumePlan {
	volumePlanLimit := types.VolumePlan{}

	volumeBindingToVolumeMap := map[[3]string]types.VolumeMap{}
	for binding, volumeMap := range volumePlan {
		volumeBindingToVolumeMap[binding.GetMapKey()] = volumeMap
	}

	for index, binding := range volumeLimit {
		if !binding.RequireSchedule() {
			continue
		}
		if volumeMap, ok := volumeBindingToVolumeMap[binding.GetMapKey()]; ok {
			volumePlanLimit[binding] = types.VolumeMap{volumeMap.GetDevice(): volumeMap.GetSize() + binding.SizeInBytes - volumeRequest[index].SizeInBytes}
		}
	}
	return volumePlanLimit
}

func getDisksLimit(volumeLimit types.VolumeBindings, volumePlanLimit types.VolumePlan, disks types.Disks) types.Disks {
	disksLimit := types.Disks{}
	for _, binding := range volumeLimit {
		if binding.RequireIOPS() && !binding.RequireSchedule() {
			disk := disks.GetDiskByPath(binding.Source)
			disksLimit.Add(types.Disks{&types.Disk{
				Device:    disk.Device,
				Mounts:    disk.Mounts,
				ReadIOPS:  binding.ReadIOPS,
				WriteIOPS: binding.WriteIOPS,
				ReadBPS:   binding.ReadBPS,
				WriteBPS:  binding.WriteBPS,
			}})
		}
	}
	for binding, volumeMap := range volumePlanLimit {
		if !binding.RequireIOPS() {
			continue
		}
		disk := disks.GetDiskByPath(volumeMap.GetDevice())
		disksLimit.Add(types.Disks{&types.Disk{
			Device:    disk.Device,
			Mounts:    disk.Mounts,
			ReadIOPS:  binding.ReadIOPS,
			WriteIOPS: binding.WriteIOPS,
			ReadBPS:   binding.ReadBPS,
			WriteBPS:  binding.WriteBPS,
		}})
	}
	return disksLimit
}

func (v *Volume) toIOPSOptions(disks types.Disks) map[string]string {
	IOPSOptions := map[string]string{}
	for _, disk := range disks {
		IOPSOptions[disk.Device] = fmt.Sprintf("%d:%d:%d:%d", disk.ReadIOPS, disk.WriteIOPS, disk.ReadBPS, disk.WriteBPS)
	}
	return IOPSOptions
}

func (v *Volume) doAlloc(resourceInfo *types.NodeResourceInfo, deployCount int, opts *types.WorkloadResourceOpts) ([]*types.EngineArgs, []*types.WorkloadResourceArgs, error) {
	// check if storage is enough
	if opts.StorageRequest > 0 {
		storageCapacity := int((resourceInfo.Capacity.Storage - resourceInfo.Usage.Storage) / opts.StorageRequest)
		if storageCapacity < deployCount {
			return nil, nil, errors.Wrapf(types.ErrInsufficientResource, "not enough storage, request: %+v, available: %+v", opts.StorageRequest, storageCapacity)
		}
	}

	resEngineArgs := []*types.EngineArgs{}
	resResourceArgs := []*types.WorkloadResourceArgs{}

	var volumePlans []types.VolumePlan
	var diskPlans []types.Disks

	// if volume scheduling is not required
	if !utils.Any(opts.VolumesRequest, func(b *types.VolumeBinding) bool { return b.RequireSchedule() || b.RequireIOPS() }) {
		for i := 0; i < deployCount; i++ {
			volumePlans = append(volumePlans, types.VolumePlan{})
			diskPlans = append(diskPlans, types.Disks{})
		}
	} else {
		volumePlans, diskPlans = schedule.GetVolumePlans(resourceInfo, opts.VolumesRequest, v.Config.Scheduler.MaxDeployCount)
		if len(volumePlans) < deployCount {
			return nil, nil, errors.Wrapf(types.ErrInsufficientResource, "not enough volume plan, need %+v, available %+v", deployCount, len(volumePlans))
		}
		volumePlans = volumePlans[:deployCount]
		diskPlans = diskPlans[:deployCount]
	}

	volumeSizeLimitMap := map[*types.VolumeBinding]int64{}
	for _, binding := range opts.VolumesLimit {
		volumeSizeLimitMap[binding] = binding.SizeInBytes
	}

	for index, volumePlan := range volumePlans {
		engineArgs := &types.EngineArgs{Storage: opts.StorageLimit}
		for _, binding := range opts.VolumesLimit.ApplyPlan(volumePlan) {
			engineArgs.Volumes = append(engineArgs.Volumes, binding.ToString(true))
		}

		volumePlanLimit := getVolumePlanLimit(opts.VolumesLimit, opts.VolumesLimit, volumePlan)
		disksLimit := getDisksLimit(opts.VolumesLimit, volumePlanLimit, resourceInfo.Capacity.Disks)

		engineArgs.IOPSOptions = v.toIOPSOptions(disksLimit)

		resourceArgs := &types.WorkloadResourceArgs{
			VolumesRequest:    opts.VolumesRequest,
			VolumesLimit:      opts.VolumesLimit,
			VolumePlanRequest: volumePlan,
			VolumePlanLimit:   volumePlanLimit,
			StorageRequest:    opts.StorageRequest,
			StorageLimit:      opts.StorageLimit,
			DisksRequest:      diskPlans[index],
			DisksLimit:        disksLimit,
		}

		resEngineArgs = append(resEngineArgs, engineArgs)
		resResourceArgs = append(resResourceArgs, resourceArgs)
	}

	return resEngineArgs, resResourceArgs, nil
}
