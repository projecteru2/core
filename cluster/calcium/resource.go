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
		nodeDetail, err := c.doGetNodeResource(ctx, node.Name, false)
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
func (c *Calcium) NodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	nr, err := c.doGetNodeResource(ctx, nodename, fix)
	if err != nil {
		return nil, err
	}
	for _, container := range nr.Containers {
		if _, err := container.Inspect(ctx); err != nil { // 用于探测节点上容器是否存在
			nr.Verification = false
			nr.Details = append(nr.Details, fmt.Sprintf("container %s inspect failed %v \n", container.ID, err))
			continue
		}
	}
	return nr, err
}

func (c *Calcium) doGetNodeResource(ctx context.Context, nodename string, fix bool) (*types.NodeResource, error) {
	var nr *types.NodeResource
	return nr, c.withNodeLocked(ctx, nodename, func(node *types.Node) error {
		containers, err := c.ListNodeContainers(ctx, node.Name, nil)
		if err != nil {
			return err
		}
		nr = &types.NodeResource{
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
		nr.VolumePercent = float64(node.VolumeUsed) / float64(node.InitVolume.Total())
		for nodeID, nmemory := range node.NUMAMemory {
			if initMemory, ok := node.InitNUMAMemory[nodeID]; ok {
				nr.NUMAMemoryPercent[nodeID] = float64(nmemory) / float64(initMemory)
			}
		}
		if cpus != node.CPUUsed {
			nr.Verification = false
			nr.Details = append(nr.Details, fmt.Sprintf("cpus used: %f diff: %f", node.CPUUsed, cpus))
		}
		node.CPU.Add(cpumap)
		for i, v := range node.CPU {
			if node.InitCPU[i] != v {
				nr.Verification = false
				nr.Details = append(nr.Details, fmt.Sprintf("cpu %s diff %d", i, node.InitCPU[i]-v))
			}
		}

		if memory+node.MemCap != node.InitMemCap {
			nr.Verification = false
			nr.Details = append(nr.Details, fmt.Sprintf("memory used: %d, diff %d", node.MemCap, node.InitMemCap-(memory+node.MemCap)))
		}

		nr.StoragePercent = 0
		if node.InitStorageCap != 0 {
			nr.StoragePercent = float64(storage) / float64(node.InitStorageCap)
			if storage+node.StorageCap != node.InitStorageCap {
				nr.Verification = false
				nr.Details = append(nr.Details, fmt.Sprintf("storage used: %d, diff %d", node.StorageCap, node.InitStorageCap-(storage+node.StorageCap)))
			}
		}

		if err := node.Engine.ResourceValidate(ctx, cpus, cpumap, memory, storage); err != nil {
			nr.Details = append(nr.Details, err.Error())
		}

		if fix {
			if err := c.doFixDiffResource(ctx, node, cpus, memory, storage); err != nil {
				log.Warnf("[doGetNodeResource] fix node resource failed %v", err)
			}
		}

		return nil
	})
}

func (c *Calcium) doFixDiffResource(ctx context.Context, node *types.Node, cpus float64, memory, storage int64) error {
	var n *types.Node
	var err error
	return utils.Txn(ctx,
		func(ctx context.Context) error {
			if n, err = c.GetNode(ctx, node.Name); err != nil {
				return err
			}
			n.CPUUsed = cpus
			for i, v := range node.CPU {
				n.CPU[i] += node.InitCPU[i] - v
			}
			n.MemCap += node.InitMemCap - (memory + node.MemCap)
			n.StorageCap += node.InitStorageCap - (storage + node.StorageCap)
			return nil
		},
		func(ctx context.Context) error {
			return c.store.UpdateNode(ctx, n)
		},
		nil,
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doAllocResource(ctx context.Context, opts *types.DeployOptions) ([]types.NodeInfo, error) {
	var err error
	var total int
	var nodesInfo []types.NodeInfo
	var nodeCPUPlans map[string][]types.CPUMap
	var nodeVolumePlans map[string][]types.VolumePlan
	return nodesInfo, c.withNodesLocked(ctx, opts.Podname, opts.Nodename, opts.NodeLabels, false, func(nodes map[string]*types.Node) error {
		if len(nodes) == 0 {
			return types.ErrInsufficientNodes
		}
		nodesInfo = getNodesInfo(nodes, opts.CPUQuota, opts.Memory, opts.Storage, opts.Volumes.TotalSize())
		// 载入之前部署的情况
		nodesInfo, err = c.store.MakeDeployStatus(ctx, opts, nodesInfo)
		if err != nil {
			return err
		}

		if !opts.CPUBind || opts.CPUQuota == 0 {
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

		volumeSchedule := false
		for _, volume := range opts.Volumes {
			if volume.RequireSchedule() {
				volumeSchedule = true
				break
			}
		}
		resourceType := types.GetResourceType(opts.CPUBind, volumeSchedule)

		switch opts.DeployMethod {
		case cluster.DeployAuto:
			nodesInfo, err = c.scheduler.CommonDivision(nodesInfo, opts.Count, total, resourceType)
		case cluster.DeployEach:
			nodesInfo, err = c.scheduler.EachDivision(nodesInfo, opts.Count, opts.NodesLimit, resourceType)
		case cluster.DeployFill:
			nodesInfo, err = c.scheduler.FillDivision(nodesInfo, opts.Count, opts.NodesLimit, resourceType)
		case cluster.DeployGlobal:
			nodesInfo, err = c.scheduler.GlobalDivision(nodesInfo, opts.Count, total, resourceType)
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
		track := -1
		return utils.Txn(
			ctx,
			func(ctx context.Context) error {
				for i, nodeInfo := range nodesInfo {
					cpuCost, quotaCost, memoryCost, storageCost, volumeCost := calcCost(
						nodeInfo, opts.Memory, opts.Storage, opts.CPUQuota, nodeCPUPlans, nodeVolumePlans,
					)
					if _, ok := nodeCPUPlans[nodeInfo.Name]; ok {
						nodesInfo[i].CPUPlan = nodeCPUPlans[nodeInfo.Name][:nodeInfo.Deploy]
					}
					if _, ok := nodeVolumePlans[nodeInfo.Name]; ok {
						nodesInfo[i].VolumePlans = nodeVolumePlans[nodeInfo.Name][:nodeInfo.Deploy]
					}
					if err = c.store.UpdateNodeResource(ctx, nodes[nodeInfo.Name], cpuCost, quotaCost, memoryCost, storageCost, volumeCost, store.ActionDecr); err != nil {
						return err // due to ctx lifecircle, this will be interrupted by client
					}
					track = i
				}
				return nil
			},
			func(ctx context.Context) error {
				go func() {
					for _, nodeInfo := range nodesInfo {
						log.Infof("[allocResource] deploy %d to %s", nodeInfo.Deploy, nodeInfo.Name)
					}
				}()
				return c.doBindProcessStatus(ctx, opts, nodesInfo)
			},
			func(ctx context.Context) error {
				for i := 0; i < track+1; i++ {
					cpuCost, quotaCost, memoryCost, storageCost, volumeCost := calcCost(
						nodesInfo[i], opts.Memory, opts.Storage, opts.CPUQuota, nodeCPUPlans, nodeVolumePlans,
					)
					if err = c.store.UpdateNodeResource(ctx, nodes[nodesInfo[i].Name], cpuCost, quotaCost, memoryCost, storageCost, volumeCost, store.ActionIncr); err != nil {
						return err
					}
				}
				return nil
			},
			c.config.GlobalTimeout,
		)
	})
}

func (c *Calcium) doBindProcessStatus(ctx context.Context, opts *types.DeployOptions, nodesInfo []types.NodeInfo) error {
	for _, nodeInfo := range nodesInfo {
		if err := c.store.SaveProcessing(ctx, opts, nodeInfo); err != nil {
			return err
		}
	}
	return nil
}

func calcCost(
	nodeInfo types.NodeInfo,
	memory, storage int64, CPUQuota float64,
	nodeCPUPlans map[string][]types.CPUMap,
	nodeVolumePlans map[string][]types.VolumePlan) (types.CPUMap, float64, int64, int64, types.VolumeMap) {
	cpuCost := types.CPUMap{}
	memoryCost := memory * int64(nodeInfo.Deploy)
	storageCost := storage * int64(nodeInfo.Deploy)
	quotaCost := CPUQuota * float64(nodeInfo.Deploy)
	volumeCost := types.VolumeMap{}

	if _, ok := nodeCPUPlans[nodeInfo.Name]; ok {
		for _, cpu := range nodeCPUPlans[nodeInfo.Name][:nodeInfo.Deploy] {
			cpuCost.Add(cpu)
		}
	}

	if _, ok := nodeVolumePlans[nodeInfo.Name]; ok {
		for _, volumePlan := range nodeVolumePlans[nodeInfo.Name][:nodeInfo.Deploy] {
			volumeCost.Add(volumePlan.IntoVolumeMap())
		}
	}

	return cpuCost, quotaCost, memoryCost, storageCost, volumeCost
}
