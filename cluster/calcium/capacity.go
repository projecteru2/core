package calcium

import (
	"context"
	"math"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// CalculateCapacity calculates capacity
func (c *Calcium) CalculateCapacity(ctx context.Context, opts *types.CalculateCapacityOptions) (msg *types.CapacityMessage, err error) {
	return msg, c.withNodesLocked(ctx, opts.Podname, opts.Nodenames, nil, false, func(nodes map[string]*types.Node) error {
		msg, err = c.calculateCapacity(ctx, opts, nodes)
		return err
	})
}

func (c *Calcium) calculateCapacity(ctx context.Context, opts *types.CalculateCapacityOptions, nodes map[string]*types.Node) (msg *types.CapacityMessage, err error) {
	nodesInfo := getNodesInfo(nodes, opts.CPUQuota, opts.Memory, opts.Storage, opts.Volumes.TotalSize())
	deployOpts := &types.DeployOptions{
		Name: opts.Appname,
		Entrypoint: &types.Entrypoint{
			Name: opts.Entryname,
		},
	}
	nodesInfo, err = c.store.MakeDeployStatus(ctx, deployOpts, nodesInfo)
	if err != nil {
		return nil, errors.WithStack(err)
	}

	msg = &types.CapacityMessage{
		NodeCapacities: make(map[string]*types.CapacityInfo),
	}

	// fill up exist
	for _, nodeInfo := range nodesInfo {
		msg.NodeCapacities[nodeInfo.Name] = &types.CapacityInfo{
			Nodename:        nodeInfo.Name,
			Exist:           nodeInfo.Count,
			CapacityCPUMem:  -1,
			CapacityVolume:  -1,
			CapacityStorage: -1,
		}
	}

	var nodeVolumePlans map[string][]types.VolumePlan

	// fill up cap cpumem
	totalCPUMem := math.MaxInt32
	if opts.CPUQuota > 0 || opts.Memory > 0 {
		for _, capacity := range msg.NodeCapacities {
			capacity.CapacityCPUMem = 0
		}
		if !opts.CPUBind || opts.CPUQuota == 0 {
			nodesInfo, totalCPUMem, err = c.scheduler.SelectMemoryNodes(nodesInfo, opts.CPUQuota, opts.Memory)
		} else {
			nodesInfo, _, totalCPUMem, err = c.scheduler.SelectCPUNodes(nodesInfo, opts.CPUQuota, opts.Memory)
		}
		if err != nil {
			return nil, errors.WithStack(err)
		}
		for _, nodeInfo := range nodesInfo {
			msg.NodeCapacities[nodeInfo.Name].CapacityCPUMem = nodeInfo.Capacity
		}
	}

	// fill up cap volume
	totalVolume := math.MaxInt32
	if len(opts.Volumes) != 0 {
		for _, capacity := range msg.NodeCapacities {
			capacity.CapacityVolume = 0
		}
		if nodesInfo, nodeVolumePlans, totalVolume, err = c.scheduler.SelectVolumeNodes(nodesInfo, opts.Volumes); err != nil {
			return nil, errors.WithStack(err)
		}
		for nodename, plans := range nodeVolumePlans {
			msg.NodeCapacities[nodename].CapacityVolume = len(plans)
		}
	}

	// fill up cap storage
	totalStorage := math.MaxInt32
	if opts.Storage > 0 {
		for _, capacity := range msg.NodeCapacities {
			capacity.CapacityStorage = 0
		}
		if nodesInfo, totalStorage, err = c.scheduler.SelectStorageNodes(nodesInfo, opts.Storage); err != nil {
			return nil, errors.WithStack(err)
		}
		for _, nodeInfo := range nodesInfo {
			msg.NodeCapacities[nodeInfo.Name].CapacityStorage = int(nodeInfo.StorageCap / opts.Storage)
		}
	}

	// sum up
	for _, nodeInfo := range nodesInfo {
		msg.NodeCapacities[nodeInfo.Name].Capacity = nodeInfo.Capacity
	}

	msg.Total = utils.Min(totalCPUMem, totalVolume, totalStorage)
	return msg, nil
}
