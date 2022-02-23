package models

import (
	"context"
	"math"

	"github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources/volume/schedule"
	"github.com/projecteru2/core/resources/volume/types"
)

// GetNodesDeployCapacity .
func (v *Volume) GetNodesDeployCapacity(ctx context.Context, nodes []string, opts *types.WorkloadResourceOpts) (map[string]*types.NodeCapacityInfo, int, error) {
	if err := opts.Validate(); err != nil {
		logrus.Errorf("[GetNodesDeployCapacity] invalid resource opts %+v, err: %v", opts, err)
		return nil, 0, err
	}

	capacityInfoMap := map[string]*types.NodeCapacityInfo{}
	total := 0
	for _, node := range nodes {
		resourceInfo, err := v.doGetNodeResourceInfo(ctx, node)
		if err != nil {
			logrus.Errorf("[GetNodesDeployCapacity] failed to get resource info of node %v, err: %v", node, err)
			return nil, 0, err
		}
		capacityInfo := v.doGetNodeCapacityInfo(ctx, node, resourceInfo, opts)
		if capacityInfo.Capacity > 0 {
			capacityInfoMap[node] = capacityInfo
			if total == math.MaxInt64 || capacityInfo.Capacity == math.MaxInt64 {
				total = math.MaxInt64
			} else {
				total += capacityInfo.Capacity
			}
		}
	}

	return capacityInfoMap, total, nil
}

func (v *Volume) doGetNodeCapacityInfo(ctx context.Context, node string, resourceInfo *types.NodeResourceInfo, opts *types.WorkloadResourceOpts) *types.NodeCapacityInfo {
	capacityInfo := &types.NodeCapacityInfo{
		Node:   node,
		Weight: 1,
	}

	// get volume capacity
	volumePlans := schedule.GetVolumePlans(resourceInfo, opts.VolumesRequest, v.config.Scheduler.MaxDeployCount)
	capacityInfo.Capacity = len(volumePlans)

	// get storage capacity
	if opts.StorageRequest > 0 {
		storageCapacity := int((resourceInfo.Capacity.Storage - resourceInfo.Usage.Storage) / opts.StorageRequest)
		if storageCapacity < capacityInfo.Capacity {
			capacityInfo.Capacity = storageCapacity
		}
	}

	// get usage and rate
	if resourceInfo.Capacity.Volumes.Total() == 0 && resourceInfo.Capacity.Storage == 0 {
		return capacityInfo
	}

	if len(opts.VolumesRequest) > 0 || opts.StorageRequest == 0 {
		capacityInfo.Usage = float64(resourceInfo.Usage.Volumes.Total()) / float64(resourceInfo.Capacity.Volumes.Total())
		capacityInfo.Rate = float64(opts.VolumesRequest.TotalSize()) / float64(resourceInfo.Capacity.Volumes.Total())
	} else if opts.StorageRequest > 0 {
		capacityInfo.Usage = float64(resourceInfo.Usage.Storage) / float64(resourceInfo.Capacity.Storage)
		capacityInfo.Rate = float64(opts.StorageRequest) / float64(resourceInfo.Capacity.Storage)
	}

	return capacityInfo
}
