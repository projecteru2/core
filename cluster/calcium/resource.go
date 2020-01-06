package calcium

import (
	"context"
	"fmt"
	"sort"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// PodResource show pod resource usage
func (c *Calcium) PodResource(ctx context.Context, podname string) (*types.PodResource, error) {
	nodes, err := c.ListPodNodes(ctx, podname, nil, true)
	if err != nil {
		return nil, err
	}
	r := &types.PodResource{
		Name:            podname,
		CPUPercents:     map[string]float64{},
		MemoryPercents:  map[string]float64{},
		StoragePercents: map[string]float64{},
		Verifications:   map[string]bool{},
		Details:         map[string]string{},
	}
	for _, node := range nodes {
		nodeDetail, err := c.doGetNodeResource(ctx, node)
		if err != nil {
			return nil, err
		}
		r.CPUPercents[node.Name] = nodeDetail.CPUPercent
		r.MemoryPercents[node.Name] = nodeDetail.MemoryPercent
		r.StoragePercents[node.Name] = nodeDetail.StoragePercent
		r.Verifications[node.Name] = nodeDetail.Verification
		r.Details[node.Name] = strings.Join(nodeDetail.Details, "\n")
	}
	return r, nil
}

// NodeResource check node's container and resource
func (c *Calcium) NodeResource(ctx context.Context, nodename string) (*types.NodeResource, error) {
	node, err := c.GetNode(ctx, nodename)
	if err != nil {
		return nil, err
	}
	nr, err := c.doGetNodeResource(ctx, node)
	if err != nil {
		return nil, err
	}
	for _, container := range nr.Containers {
		_, err := container.Inspect(ctx) // 用于探测节点上容器是否存在
		if err != nil {
			nr.Verification = false
			nr.Details = append(nr.Details, fmt.Sprintf("container %s inspect failed %v \n", container.ID, err))
			continue
		}
	}
	return nr, err
}

func (c *Calcium) doGetNodeResource(ctx context.Context, node *types.Node) (*types.NodeResource, error) {
	containers, err := c.ListNodeContainers(ctx, node.Name, nil)
	if err != nil {
		return nil, err
	}
	nr := &types.NodeResource{
		Name: node.Name, CPU: node.CPU, MemCap: node.MemCap, StorageCap: node.StorageCap,
		Containers: containers, Verification: true, Details: []string{},
	}
	cpus := 0.0
	memory := int64(0)
	storage := int64(0)
	cpumap := types.CPUMap{}
	for _, container := range containers {
		cpus = utils.Round(cpus + container.Quota)
		memory += container.Memory
		storage += container.Storage
		cpumap.Add(container.CPU)
	}
	nr.CPUPercent = cpus / float64(len(node.InitCPU))
	nr.MemoryPercent = float64(memory) / float64(node.InitMemCap)
	nr.NUMAMemoryPercent = map[string]float64{}
	nr.VolumePercent = float64(node.VolumeUsed) / float64(len(node.InitVolume))
	for nodeID, nmemory := range node.NUMAMemory {
		if initMemory, ok := node.InitNUMAMemory[nodeID]; ok {
			nr.NUMAMemoryPercent[nodeID] = float64(nmemory) / float64(initMemory)
		}
	}
	if cpus != node.CPUUsed {
		nr.Verification = false
		nr.Details = append(nr.Details, fmt.Sprintf("cpus used record: %f but now: %f", node.CPUUsed, cpus))
	}
	node.CPU.Add(cpumap)
	for i, v := range node.CPU {
		if node.InitCPU[i] != v {
			nr.Verification = false
			nr.Details = append(nr.Details, fmt.Sprintf("cpu %s now %d", i, v))
		}
	}

	if memory+node.MemCap != node.InitMemCap {
		nr.Verification = false
		nr.Details = append(nr.Details, fmt.Sprintf("memory now %d", node.InitMemCap-(memory+node.MemCap)))
	}

	nr.StoragePercent = 0
	if node.InitStorageCap != 0 {
		nr.StoragePercent = float64(storage) / float64(node.InitStorageCap)
		if storage+node.StorageCap != node.InitStorageCap {
			nr.Verification = false
			nr.Details = append(nr.Details, fmt.Sprintf("storage now %d", node.InitStorageCap-(storage+node.StorageCap)))
		}
	}

	if err := node.Engine.ResourceValidate(ctx, cpus, cpumap, memory, storage); err != nil {
		nr.Details = append(nr.Details, err.Error())
	}

	return nr, nil
}

func (c *Calcium) doAllocResource(ctx context.Context, opts *types.DeployOptions) ([]types.NodeInfo, error) {
	var err error
	var total int
	var nodesInfo []types.NodeInfo
	var nodeCPUPlans map[string][]types.CPUMap
	var nodeVolumePlans map[string][]types.VolumePlan
	if err = c.withNodesLocked(ctx, opts.Podname, opts.Nodename, opts.NodeLabels, false, func(nodes map[string]*types.Node) error {
		if len(nodes) == 0 {
			return types.ErrInsufficientNodes
		}
		nodesInfo = getNodesInfo(nodes, opts.CPUQuota, opts.Memory, opts.Storage)
		// 载入之前部署的情况
		nodesInfo, err = c.store.MakeDeployStatus(ctx, opts, nodesInfo)
		if err != nil {
			return err
		}

		if !opts.CPUBind {
			nodesInfo, total, err = c.scheduler.SelectMemoryNodes(nodesInfo, opts.CPUQuota, opts.Memory) // 还是以 Bytes 作单位， 不转换了
		} else {
			log.Info("[doAllocResource] CPU Bind, selecting CPU plan")
			nodesInfo, nodeCPUPlans, total, err = c.scheduler.SelectCPUNodes(nodesInfo, opts.CPUQuota, opts.Memory)
		}
		if err != nil {
			return err
		}

		var storTotal int
		if nodesInfo, storTotal, err = c.scheduler.SelectStorageNodes(nodesInfo, opts.Storage); err != nil {
			return err
		}

		var volumeTotal int
		if nodesInfo, nodeVolumePlans, volumeTotal, err = c.scheduler.SelectVolumeNodes(nodesInfo, opts.Volumes); err != nil {
			return err
		}

		total = utils.Min(volumeTotal, storTotal, total)

		switch opts.DeployMethod {
		case cluster.DeployAuto:
			nodesInfo, err = c.scheduler.CommonDivision(nodesInfo, opts.Count, total)
		case cluster.DeployEach:
			nodesInfo, err = c.scheduler.EachDivision(nodesInfo, opts.Count, opts.NodesLimit)
		case cluster.DeployFill:
			nodesInfo, err = c.scheduler.FillDivision(nodesInfo, opts.Count, opts.NodesLimit)
		case cluster.DeployGlobal:
			nodesInfo, err = c.scheduler.GlobalDivision(nodesInfo, opts.Count, total)
		default:
			return types.ErrBadDeployMethod
		}
		if err != nil {
			return err
		}

		// 资源处理
		sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Deploy < nodesInfo[j].Deploy })
		p := sort.Search(len(nodesInfo), func(i int) bool { return nodesInfo[i].Deploy > 0 })
		// p 最大也就是 len(nodesInfo) - 1
		if p == len(nodesInfo) {
			return types.ErrInsufficientRes
		}
		nodesInfo = nodesInfo[p:]
		for i, nodeInfo := range nodesInfo {
			cpuCost := types.CPUMap{}
			memoryCost := opts.Memory * int64(nodeInfo.Deploy)
			storageCost := opts.Storage * int64(nodeInfo.Deploy)
			quotaCost := opts.CPUQuota * float64(nodeInfo.Deploy)
			volumeCost := types.VolumeMap{}

			if _, ok := nodeCPUPlans[nodeInfo.Name]; ok {
				cpuList := nodeCPUPlans[nodeInfo.Name][:nodeInfo.Deploy]
				nodesInfo[i].CPUPlan = cpuList
				for _, cpu := range cpuList {
					cpuCost.Add(cpu)
				}
			}

			if _, ok := nodeVolumePlans[nodeInfo.Name]; ok {
				nodesInfo[i].VolumePlans = nodeVolumePlans[nodeInfo.Name][:nodeInfo.Deploy]
				for _, volumePlan := range nodesInfo[i].VolumePlans {
					volumeCost.Add(volumePlan.Merge())
				}
			}

			if err = c.store.UpdateNodeResource(ctx, nodes[nodeInfo.Name], cpuCost, quotaCost, memoryCost, storageCost, volumeCost, store.ActionDecr); err != nil {
				return err
			}
		}
		go func() {
			for _, nodeInfo := range nodesInfo {
				log.Infof("[allocResource] deploy %d to %s", nodeInfo.Deploy, nodeInfo.Name)
			}
		}()
		return nil
	}); err != nil {
		return nil, err
	}

	return nodesInfo, c.doBindProcessStatus(ctx, opts, nodesInfo)
}

func (c *Calcium) doBindProcessStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) error {
	for _, nodeInfo := range nodesInfo {
		if err := c.store.SaveProcessing(ctx, opts, nodeInfo); err != nil {
			return err
		}
	}
	return nil
}
