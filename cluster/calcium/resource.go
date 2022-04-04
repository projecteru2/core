package calcium

import (
	"context"
	"fmt"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// PodResource show pod resource usage
func (c *Calcium) PodResource(ctx context.Context, podname string) (chan *types.NodeResource, error) {
	logger := log.WithField("Calcium", "PodResource").WithField("podname", podname)
	nodeCh, err := c.ListPodNodes(ctx, &types.ListNodesOptions{Podname: podname, All: true})
	if err != nil {
		return nil, logger.Err(ctx, err)
	}
	ch := make(chan *types.NodeResource)
	pool := utils.NewGoroutinePool(int(c.config.MaxConcurrency))
	go func() {
		defer close(ch)
		for node := range nodeCh {
			nodename := node.Name
			pool.Go(ctx, func() {
				nodeResource, err := c.doGetNodeResource(ctx, nodename, false)
				if err != nil {
					nodeResource = &types.NodeResource{
						Name: nodename, Diffs: []string{logger.Err(ctx, err).Error()},
					}
				}
				ch <- nodeResource
			})
		}
		pool.Wait(ctx)
	}()
	return ch, nil
}

// NodeResource check node's workload and resource
func (c *Calcium) NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	logger := log.WithField("Calcium", "NodeResource").WithField("nodename", nodename).WithField("fix", fix)
	if nodename == "" {
		return nil, logger.Err(ctx, types.ErrEmptyNodeName)
	}

	nr, err := c.doGetNodeResource(ctx, nodename, fix)
	if err != nil {
		return nil, logger.Err(ctx, err)
	}
	for _, workload := range nr.Workloads {
		if _, err := workload.Inspect(ctx); err != nil { // 用于探测节点上容器是否存在
			nr.Diffs = append(nr.Diffs, fmt.Sprintf("workload %s inspect failed %v \n", workload.ID, err))
			continue
		}
	}
	return nr, logger.Err(ctx, err)
}

func (c *Calcium) doGetNodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	var nr *types.NodeResource
	return nr, c.withNodePodLocked(ctx, nodename, func(ctx context.Context, node *types.Node) error {
		workloads, err := c.ListNodeWorkloads(ctx, node.Name, nil)
		if err != nil {
			return err
		}
		nr = &types.NodeResource{
			Name: node.Name, CPU: node.CPU, MemCap: node.MemCap, StorageCap: node.StorageCap,
			Workloads: workloads, Diffs: []string{},
		}

		cpuByWorkloads := 0.0
		memoryByWorkloads := int64(0)
		storageByWorkloads := int64(0) // volume inclusive
		cpumapByWorkloads := types.CPUMap{}
		volumeByWorkloads := int64(0)
		volumeMapByWorkloads := types.VolumeMap{}
		monopolyVolumeByWorkloads := map[string][]string{} // volume -> array of workload id
		for _, workload := range workloads {
			cpuByWorkloads = utils.Round(cpuByWorkloads + workload.CPUQuotaRequest)
			memoryByWorkloads += workload.MemoryRequest
			storageByWorkloads += workload.StorageRequest
			cpumapByWorkloads.Add(workload.CPU)
			for vb, vmap := range workload.VolumePlanRequest {
				volumeByWorkloads += vmap.Total()
				volumeMapByWorkloads.Add(vmap)
				if vb.RequireScheduleMonopoly() {
					monopolyVolumeByWorkloads[vmap.GetResourceID()] = append(monopolyVolumeByWorkloads[vmap.GetResourceID()], workload.ID)
				}
			}
		}
		nr.CPUPercent = cpuByWorkloads / float64(len(node.InitCPU))
		nr.MemoryPercent = float64(memoryByWorkloads) / float64(node.InitMemCap)
		nr.NUMAMemoryPercent = map[string]float64{}
		nr.VolumePercent = float64(node.VolumeUsed) / float64(node.InitVolume.Total())
		for nodeID, nmemory := range node.NUMAMemory {
			if initMemory, ok := node.InitNUMAMemory[nodeID]; ok {
				nr.NUMAMemoryPercent[nodeID] = float64(nmemory) / float64(initMemory)
			}
		}

		// cpu
		if cpuByWorkloads != node.CPUUsed {
			nr.Diffs = append(nr.Diffs,
				fmt.Sprintf("node.CPUUsed != sum(workload.CPURequest): %.2f != %.2f", node.CPUUsed, cpuByWorkloads))
		}
		node.CPU.Add(cpumapByWorkloads)
		for i, v := range node.CPU {
			if node.InitCPU[i] != v {
				nr.Diffs = append(nr.Diffs,
					fmt.Sprintf("\tsum(workload.CPU[%s]) + node.CPU[%s] != node.InitCPU[%s]: %d + %d != %d", i, i, i,
						cpumapByWorkloads[i], node.CPU[i]-cpumapByWorkloads[i], node.InitCPU[i]))
			}
		}

		// memory
		if memoryByWorkloads+node.MemCap != node.InitMemCap {
			nr.Diffs = append(nr.Diffs,
				fmt.Sprintf("node.MemCap + sum(workload.memoryRequest) != node.InitMemCap: %d + %d != %d",
					node.MemCap, memoryByWorkloads, node.InitMemCap))
		}

		// storage
		nr.StoragePercent = 0
		if node.InitStorageCap != 0 {
			nr.StoragePercent = float64(storageByWorkloads) / float64(node.InitStorageCap)
		}
		if storageByWorkloads+node.StorageCap != node.InitStorageCap {
			nr.Diffs = append(nr.Diffs, fmt.Sprintf("sum(workload.storageRequest) + node.StorageCap != node.InitStorageCap: %d + %d != %d", storageByWorkloads, node.StorageCap, node.InitStorageCap))
		}

		// volume
		if node.VolumeUsed != volumeByWorkloads {
			nr.Diffs = append(nr.Diffs, fmt.Sprintf("node.VolumeUsed != sum(workload.VolumeRequest): %d != %d", node.VolumeUsed, volumeByWorkloads))
		}
		node.Volume.Add(volumeMapByWorkloads)
		for i, v := range node.Volume {
			if node.InitVolume[i] != v {
				nr.Diffs = append(nr.Diffs, fmt.Sprintf("\tsum(workload.Volume[%s]) + node.Volume[%s] != node.InitVolume[%s]: %d + %d != %d",
					i, i, i,
					volumeMapByWorkloads[i], node.Volume[i]-volumeMapByWorkloads[i], node.InitVolume[i]))
			}
		}
		for vol, ids := range monopolyVolumeByWorkloads {
			idx := utils.Unique(ids, func(i int) string { return ids[i] })
			if len(ids[:idx]) > 1 {
				nr.Diffs = append(nr.Diffs, fmt.Sprintf("\tmonopoly volume used by multiple workloads: %s, %+v", vol, ids))
			}
		}

		// volume and storage
		if node.InitStorageCap < node.InitVolume.Total() {
			nr.Diffs = append(nr.Diffs, fmt.Sprintf("init storage < init volumes: %d < %d", node.InitStorageCap, node.InitVolume.Total()))
		}

		if err := node.Engine.ResourceValidate(ctx, cpuByWorkloads, cpumapByWorkloads, memoryByWorkloads, storageByWorkloads); err != nil {
			nr.Diffs = append(nr.Diffs, err.Error())
		}

		if fix {
			if err := c.doFixDiffResource(ctx, node, cpuByWorkloads, memoryByWorkloads, storageByWorkloads, volumeByWorkloads); err != nil {
				log.Warnf(ctx, "[doGetNodeResource] fix node resource failed %v", err)
			}
		}

		return nil
	})
}

func (c *Calcium) doFixDiffResource(ctx context.Context, node *types.Node, cpuByWorkloads float64, memoryByWorkloads, storageByWorkloads, volumeByWorkloads int64) (err error) {
	var n *types.Node
	if n, err = c.GetNode(ctx, node.Name); err != nil {
		return err
	}
	n.CPUUsed = cpuByWorkloads
	for i, v := range node.CPU {
		n.CPU[i] += node.InitCPU[i] - v
	}
	n.MemCap = node.InitMemCap - memoryByWorkloads
	if n.InitStorageCap < n.InitVolume.Total() {
		n.InitStorageCap = n.InitVolume.Total()
	}
	n.StorageCap = n.InitStorageCap - storageByWorkloads
	n.VolumeUsed = volumeByWorkloads
	for i, v := range node.Volume {
		n.Volume[i] += node.InitVolume[i] - v
	}
	return errors.WithStack(c.store.UpdateNodes(ctx, n))
}

func (c *Calcium) doAllocResource(ctx context.Context, nodeMap map[string]*types.Node, opts *types.DeployOptions) ([]resourcetypes.ResourcePlans, map[string]int, error) {
	total, plans, strategyInfos, err := c.doCalculateCapacity(ctx, nodeMap, opts)
	if err != nil {
		return nil, nil, err
	}
	if err = c.store.MakeDeployStatus(ctx, opts.Name, opts.Entrypoint.Name, strategyInfos); err != nil {
		return nil, nil, errors.WithStack(err)
	}
	deployMap, err := strategy.Deploy(ctx, opts, strategyInfos, total)
	if err != nil {
		return nil, nil, err
	}
	log.Infof(ctx, "[Calium.doAllocResource] deployMap: %+v", deployMap)
	return plans, deployMap, nil
}

// called on changes of resource binding, such as cpu binding
// as an internal api, remap doesn't lock node, the responsibility of that should be taken on by caller
func (c *Calcium) remapResource(ctx context.Context, node *types.Node) (ch <-chan enginetypes.VirtualizationRemapMessage, err error) {
	workloads, err := c.store.ListNodeWorkloads(ctx, node.Name, nil)
	if err != nil {
		return
	}
	remapOpts := &enginetypes.VirtualizationRemapOptions{
		CPUAvailable:      node.CPU,
		CPUInit:           node.InitCPU,
		CPUShareBase:      int64(c.config.Scheduler.ShareBase),
		WorkloadResources: make(map[string]enginetypes.VirtualizationResource),
	}
	for _, workload := range workloads {
		remapOpts.WorkloadResources[workload.ID] = enginetypes.VirtualizationResource{
			CPU:      workload.CPU,
			Quota:    workload.CPUQuotaLimit,
			NUMANode: workload.NUMANode,
		}
	}
	ch, err = node.Engine.VirtualizationResourceRemap(ctx, remapOpts)
	return ch, errors.WithStack(err)
}

func (c *Calcium) doRemapResourceAndLog(ctx context.Context, logger log.Fields, node *types.Node) {
	log.Debugf(ctx, "[doRemapResourceAndLog] remap node %s", node.Name)
	ctx, cancel := context.WithTimeout(utils.InheritTracingInfo(ctx, context.TODO()), c.config.GlobalTimeout)
	defer cancel()

	err := c.withNodeOperationLocked(ctx, node.Name, func(ctx context.Context, node *types.Node) error {
		logger = logger.WithField("Calcium", "doRemapResourceAndLog").WithField("nodename", node.Name)
		if ch, err := c.remapResource(ctx, node); logger.Err(ctx, err) == nil {
			for msg := range ch {
				logger.WithField("id", msg.ID).Err(ctx, msg.Error) // nolint:errcheck
			}
		}
		return nil
	})

	if err != nil {
		log.Errorf(ctx, "[doRemapResourceAndLog] remap node %s failed, err: %v", node.Name, err)
	}
}
