package models

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanity-io/litter"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources/volume/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const NodeResourceInfoKey = "/resource/volume/%s"

// GetNodeResourceInfo .
func (v *Volume) GetNodeResourceInfo(ctx context.Context, node string, workloadResourceMap *types.WorkloadResourceArgsMap, fix bool) (*types.NodeResourceInfo, []string, error) {
	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		return nil, nil, err
	}

	diffs := []string{}

	totalVolumeMap := types.VolumeMap{}
	totalStorageUsage := int64(0)
	totalDiskUsage := types.Disks{}

	if workloadResourceMap != nil {
		for _, args := range *workloadResourceMap {
			for _, volumeMap := range args.VolumePlanRequest {
				totalVolumeMap.Add(volumeMap)
			}
			totalStorageUsage += args.StorageRequest
			totalDiskUsage.Add(args.DisksRequest.RemoveMounts())
		}
	}

	if resourceInfo.Usage.Storage != totalStorageUsage {
		diffs = append(diffs, fmt.Sprintf("node.Storage != sum(workload.Storage): %+v != %+v", resourceInfo.Usage.Storage, totalStorageUsage))
	}

	for volume, size := range resourceInfo.Usage.Volumes {
		if totalVolumeMap[volume] != size {
			diffs = append(diffs, fmt.Sprintf("node.Volumes[%s] != sum(workload.Volumes[%s]): %+v != %+v", volume, volume, size, totalVolumeMap[volume]))
		}
	}
	for volume, size := range totalVolumeMap {
		if vol, ok := resourceInfo.Usage.Volumes[volume]; !ok && vol != size {
			diffs = append(diffs, fmt.Sprintf("node.Volumes[%s] != sum(workload.Volumes[%s]): %+v != %+v", volume, volume, resourceInfo.Usage.Volumes[volume], size))
		}
	}
	for _, disk := range resourceInfo.Usage.Disks {
		d := totalDiskUsage.GetDiskByDevice(disk.Device)
		if d == nil {
			d = &types.Disk{
				Device:    disk.Device,
				Mounts:    disk.Mounts,
				ReadIOPS:  0,
				WriteIOPS: 0,
				ReadBPS:   0,
				WriteBPS:  0,
			}
		}
		d.Mounts = disk.Mounts
		computedDisk := d.String()
		storedDisk := disk.String()
		if computedDisk != storedDisk {
			diffs = append(diffs, fmt.Sprintf("node.Disks[%s] != sum(workload.Disks[%s]): %+v != %+v", disk.Device, disk.Device, storedDisk, computedDisk))
		}
	}

	if fix {
		resourceInfo.Usage = &types.NodeResourceArgs{
			Volumes: totalVolumeMap,
			Storage: totalStorageUsage,
			Disks:   totalDiskUsage,
		}
		if err = v.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
			log.Warnf(ctx, "[GetNodeResourceInfo] failed to fix node resource, err: %+v", err)
			diffs = append(diffs, "fix failed")
		}
	}

	return resourceInfo, diffs, nil
}

// priority: node resource opts > node resource args > workload resource args list
func (v *Volume) calculateNodeResourceArgs(origin *types.NodeResourceArgs, nodeResourceOpts *types.NodeResourceOpts, nodeResourceArgs *types.NodeResourceArgs, workloadResourceArgs []*types.WorkloadResourceArgs, delta bool, incr bool) (res *types.NodeResourceArgs) {
	if origin == nil || !delta {
		res = (&types.NodeResourceArgs{}).DeepCopy()
	} else {
		res = origin.DeepCopy()
	}

	if nodeResourceOpts != nil {
		nodeResourceArgs := &types.NodeResourceArgs{
			Volumes: nodeResourceOpts.Volumes,
			Storage: nodeResourceOpts.Storage,
			Disks:   nodeResourceOpts.Disks,
		}

		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}
		return res
	}

	if nodeResourceArgs != nil {
		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}
		return res
	}

	for _, args := range workloadResourceArgs {
		nodeResourceArgs := &types.NodeResourceArgs{
			Volumes: map[string]int64{},
			Storage: args.StorageRequest,
			Disks:   args.DisksRequest,
		}
		for _, volumeMap := range args.VolumePlanRequest {
			nodeResourceArgs.Volumes.Add(volumeMap)
		}
		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}
	}
	return res
}

// SetNodeResourceUsage .
func (v *Volume) SetNodeResourceUsage(ctx context.Context, node string, nodeResourceOpts *types.NodeResourceOpts, nodeResourceArgs *types.NodeResourceArgs, workloadResourceArgs []*types.WorkloadResourceArgs, delta bool, incr bool) (before *types.NodeResourceArgs, after *types.NodeResourceArgs, err error) {
	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		log.Errorf(ctx, err, "[SetNodeResourceInfo] failed to get resource info of node %+v", node)
		return nil, nil, err
	}

	before = resourceInfo.Usage.DeepCopy()
	resourceInfo.Usage = v.calculateNodeResourceArgs(resourceInfo.Usage, nodeResourceOpts, nodeResourceArgs, workloadResourceArgs, delta, incr)

	if err := v.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
		return nil, nil, err
	}
	return before, resourceInfo.Usage, nil
}

// SetNodeResourceCapacity .
func (v *Volume) SetNodeResourceCapacity(ctx context.Context, node string, nodeResourceOpts *types.NodeResourceOpts, nodeResourceArgs *types.NodeResourceArgs, delta bool, incr bool) (before *types.NodeResourceArgs, after *types.NodeResourceArgs, err error) {
	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		log.Errorf(ctx, err, "[SetNodeResourceInfo] failed to get resource info of node %+v", node)
		return nil, nil, err
	}

	before = resourceInfo.Capacity.DeepCopy()
	if nodeResourceOpts != nil {
		if len(nodeResourceOpts.RMDisks) > 0 {
			if delta {
				return nil, nil, coretypes.ErrInvalidEngineArgs
			}
			rmDisksMap := map[string]struct{}{}
			for _, rmDisk := range nodeResourceOpts.RMDisks {
				rmDisksMap[rmDisk] = struct{}{}
			}
			resourceInfo.Capacity.Disks = utils.Filter(resourceInfo.Capacity.Disks, func(d *types.Disk) bool {
				_, ok := rmDisksMap[d.Device]
				return !ok
			})
		}
		if !delta {
			nodeResourceOpts.SkipEmpty(resourceInfo.Capacity)
		}
	}
	resourceInfo.Capacity = v.calculateNodeResourceArgs(resourceInfo.Capacity, nodeResourceOpts, nodeResourceArgs, nil, delta, incr)
	if delta {
		resourceInfo.Capacity.RemoveEmpty()
	}

	if err := v.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
		return nil, nil, err
	}
	return before, resourceInfo.Capacity, nil
}

// SetNodeResourceInfo .
func (v *Volume) SetNodeResourceInfo(ctx context.Context, node string, resourceCapacity *types.NodeResourceArgs, resourceUsage *types.NodeResourceArgs) error {
	resourceInfo := &types.NodeResourceInfo{
		Capacity: resourceCapacity,
		Usage:    resourceUsage,
	}

	return v.doSetNodeResourceInfo(ctx, node, resourceInfo)
}

func (v *Volume) doGetNodeResourceInfo(ctx context.Context, node string) (*types.NodeResourceInfo, error) {
	resourceInfo := &types.NodeResourceInfo{}
	resp, err := v.store.GetOne(ctx, fmt.Sprintf(NodeResourceInfoKey, node))
	if err != nil {
		log.Errorf(ctx, err, "[doGetNodeResourceInfo] failed to get node resource info of node %+v", node)
		return nil, err
	}
	if err = json.Unmarshal(resp.Value, resourceInfo); err != nil {
		log.Errorf(ctx, err, "[doGetNodeResourceInfo] failed to unmarshal node resource info of node %+v", node)
		return nil, err
	}
	return resourceInfo, nil
}

func (v *Volume) doSetNodeResourceInfo(ctx context.Context, node string, resourceInfo *types.NodeResourceInfo) error {
	if err := resourceInfo.Validate(); err != nil {
		log.Errorf(ctx, err, "[doSetNodeResourceInfo] invalid resource info %+v", litter.Sdump(resourceInfo))
		return err
	}

	data, err := json.Marshal(resourceInfo)
	if err != nil {
		log.Errorf(ctx, err, "[doSetNodeResourceInfo] faield to marshal resource info %+v", resourceInfo)
		return err
	}

	if _, err = v.store.Put(ctx, fmt.Sprintf(NodeResourceInfoKey, node), string(data)); err != nil {
		log.Errorf(ctx, err, "[doSetNodeResourceInfo] faield to put resource info %+v", resourceInfo)
		return err
	}
	return nil
}
