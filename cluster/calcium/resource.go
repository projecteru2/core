package calcium

import (
	"context"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) allocResource(ctx context.Context, opts *types.DeployOptions, podType string) ([]types.NodeInfo, error) {
	var err error
	var total int
	var nodesInfo []types.NodeInfo
	var nodeCPUPlans map[string][]types.CPUMap

	lock, err := c.Lock(ctx, opts.Podname, c.config.LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock(ctx)

	cpuandmem, err := c.getCPUAndMem(ctx, opts.Podname, opts.Nodename, opts.NodeLabels)
	if err != nil {
		return nil, err
	}
	nodesInfo = getNodesInfo(cpuandmem)

	// 载入之前部署的情况
	nodesInfo, err = c.store.MakeDeployStatus(ctx, opts, nodesInfo)
	if err != nil {
		return nil, err
	}

	switch podType {
	case scheduler.MEMORY_PRIOR:
		log.Debugf("[allocResource] Input opts.CPUQuota: %f", opts.CPUQuota)
		nodesInfo, total, err = c.scheduler.SelectMemoryNodes(nodesInfo, opts.CPUQuota, opts.Memory) // 还是以 Bytes 作单位， 不转换了
	case scheduler.CPU_PRIOR:
		nodesInfo, nodeCPUPlans, total, err = c.scheduler.SelectCPUNodes(nodesInfo, opts.CPUQuota, opts.Memory)
	default:
		return nil, types.ErrBadPodType
	}
	if err != nil {
		return nil, err
	}

	switch opts.DeployMethod {
	case cluster.DeployAuto:
		nodesInfo, err = c.scheduler.CommonDivision(nodesInfo, opts.Count, total)
	case cluster.DeployEach:
		nodesInfo, err = c.scheduler.EachDivision(nodesInfo, opts.Count, total)
	case cluster.DeployFill:
		nodesInfo, err = c.scheduler.FillDivision(nodesInfo, opts.Count, total)
	default:
		return nil, types.ErrBadDeployMethod
	}
	if err != nil {
		return nil, err
	}

	// 资源处理
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].Deploy < nodesInfo[j].Deploy })
	p := sort.Search(len(nodesInfo), func(i int) bool { return nodesInfo[i].Deploy > 0 })
	// p 最大也就是 len(nodesInfo) - 1
	if p == len(nodesInfo) {
		return nil, types.ErrInsufficientRes
	}
	nodesInfo = nodesInfo[p:]
	for i, nodeInfo := range nodesInfo {
		cpuCost := types.CPUMap{}
		memoryCost := opts.Memory * int64(nodeInfo.Deploy)

		if _, ok := nodeCPUPlans[nodeInfo.Name]; ok {
			cpuList := nodeCPUPlans[nodeInfo.Name][:nodeInfo.Deploy]
			nodesInfo[i].CPUPlan = cpuList
			for _, cpu := range cpuList {
				cpuCost.Add(cpu)
			}
		}
		if err := c.store.UpdateNodeResource(ctx, opts.Podname, nodeInfo.Name, cpuCost, memoryCost, "-"); err != nil {
			return nil, err
		}
	}
	log.Debugf("[allocResource] result %v", nodesInfo)
	return nodesInfo, nil
}

func (c *Calcium) getCPUAndMem(ctx context.Context, podname, nodename string, labels map[string]string) (map[string]types.CPUAndMem, error) {
	result := make(map[string]types.CPUAndMem)
	var nodes []*types.Node
	var err error
	if nodename == "" {
		nodes, err = c.ListPodNodes(ctx, podname, false)
		if err != nil {
			return result, err
		}
		nodeList := []*types.Node{}
		for _, node := range nodes {
			if filterNode(node, labels) {
				nodeList = append(nodeList, node)
			}
		}
		nodes = nodeList
	} else {
		n, err := c.GetNode(ctx, podname, nodename)
		if err != nil {
			return result, err
		}
		nodes = append(nodes, n)
	}

	if len(nodes) == 0 {
		return result, types.ErrInsufficientNodes
	}

	result = makeCPUAndMem(nodes)
	go metrics.Client.SendMemCap(result)
	return result, nil
}
