package models

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sanity-io/litter"
	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/volume/types"
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

	for _, args := range *workloadResourceMap {
		for _, volumeMap := range args.VolumePlanRequest {
			totalVolumeMap.Add(volumeMap)
		}
		totalStorageUsage += args.StorageRequest
	}

	if resourceInfo.Usage.Storage != totalStorageUsage {
		diffs = append(diffs, fmt.Sprintf("node.Storage != sum(workload.Storage): %v != %v", resourceInfo.Usage.Storage, totalStorageUsage))
	}

	for volume, size := range totalVolumeMap {
		if resourceInfo.Usage.Volumes[volume] != size {
			diffs = append(diffs, fmt.Sprintf("node.Volumes[%v] != sum(workload.Volumes[%v]: %v != %v)", volume, volume, resourceInfo.Usage.Volumes[volume], size))
		}
	}

	if fix {
		resourceInfo.Usage = &types.NodeResourceArgs{
			Volumes: totalVolumeMap,
			Storage: totalStorageUsage,
		}
		if err = v.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
			logrus.Warnf("[GetNodeResourceInfo] failed to fix node resource, err: %v", err)
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
			Storage: nodeResourceOpts.Storage + nodeResourceOpts.Volumes.Total(),
		}

		if incr {
			res.Add(nodeResourceArgs)
		} else {
			res.Sub(nodeResourceArgs)
		}

		//e.g. `--volume /data1:0` means to remove `/data1`
		res.RemoveEmpty(nodeResourceArgs)
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
		logrus.Errorf("[SetNodeResourceInfo] failed to get resource info of node %v, err: %v", node, err)
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
		logrus.Errorf("[SetNodeResourceInfo] failed to get resource info of node %v, err: %v", node, err)
		return nil, nil, err
	}

	before = resourceInfo.Capacity.DeepCopy()
	if !delta {
		nodeResourceOpts.SkipEmpty(resourceInfo.Capacity)
	}
	resourceInfo.Capacity = v.calculateNodeResourceArgs(resourceInfo.Usage, nodeResourceOpts, nodeResourceArgs, nil, delta, incr)

	if err := v.doSetNodeResourceInfo(ctx, node, resourceInfo); err != nil {
		return nil, nil, err
	}
	return before, resourceInfo.Capacity, nil
}

// SetNodeResourceInfo .
func (v *Volume) SetNodeResourceInfo(ctx context.Context, node string, resourceCapacity *types.NodeResourceArgs, resourceUsage *types.NodeResourceArgs) error {
	resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
	if err != nil {
		logrus.Errorf("[SetNodeResourceInfo] failed to get resource info of node %v, err: %v", node, err)
		return err
	}

	resourceInfo.Capacity = resourceCapacity
	resourceInfo.Usage = resourceUsage

	return v.doSetNodeResourceInfo(ctx, node, resourceInfo)
}

func (v *Volume) doGetNodeResourceInfo(ctx context.Context, node string) (*types.NodeResourceInfo, error) {
	resourceInfo := &types.NodeResourceInfo{}
	resp, err := v.store.GetOne(ctx, fmt.Sprintf(NodeResourceInfoKey, node))
	if err != nil {
		logrus.Errorf("[doGetNodeResourceInfo] failed to get node resource info of node %v, err: %v", node, err)
		return nil, err
	}
	if err = json.Unmarshal(resp.Value, resourceInfo); err != nil {
		logrus.Errorf("[doGetNodeResourceInfo] failed to unmarshal node resource info of node %v, err: %v", node, err)
		return nil, err
	}
	return resourceInfo, nil
}

func (v *Volume) doSetNodeResourceInfo(ctx context.Context, node string, resourceInfo *types.NodeResourceInfo) error {
	if err := resourceInfo.Validate(); err != nil {
		logrus.Errorf("[doSetNodeResourceInfo] invalid resource info %v, err: %v", litter.Sdump(resourceInfo), err)
		return err
	}

	data, err := json.Marshal(resourceInfo)
	if err != nil {
		logrus.Errorf("[doSetNodeResourceInfo] faield to marshal resource info %+v, err: %v", resourceInfo, err)
		return err
	}

	if _, err = v.store.Put(ctx, fmt.Sprintf(NodeResourceInfoKey, node), string(data)); err != nil {
		logrus.Errorf("[doSetNodeResourceInfo] faield to put resource info %+v, err: %v", resourceInfo, err)
		return err
	}
	return nil
}
