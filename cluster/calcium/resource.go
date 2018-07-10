package calcium

import (
	"context"
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/stats"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) allocResource(ctx context.Context, opts *types.DeployOptions, podType string) (map[string][]types.CPUMap, []types.NodeInfo, error) {
	var err error
	var total int
	var nodesInfo []types.NodeInfo
	var nodeCPUPlans map[string][]types.CPUMap

	lock, err := c.Lock(ctx, opts.Podname, c.config.LockTimeout)
	if err != nil {
		return nil, nil, err
	}
	defer lock.Unlock(ctx)

	cpuandmem, nodes, err := c.getCPUAndMem(ctx, opts.Podname, opts.Nodename, opts.NodeLabels)
	if err != nil {
		return nil, nil, err
	}
	nodesInfo = getNodesInfo(cpuandmem)

	// 载入之前部署的情况
	nodesInfo, err = c.store.MakeDeployStatus(ctx, opts, nodesInfo)
	if err != nil {
		return nil, nil, err
	}

	switch podType {
	case scheduler.MEMORY_PRIOR:
		// Raw
		if opts.RawResource {
			for i := range nodesInfo {
				nodesInfo[i].Capacity = opts.Count
			}
			nodesInfo, err = c.scheduler.CommonDivision(nodesInfo, opts.Count, len(nodesInfo)*opts.Count)
			return nil, nodesInfo, err
		}
		cpuRate := int64(opts.CPUQuota * float64(cluster.CPUPeriodBase))
		log.Debugf("[allocResource] Input opts.CPUQuota: %f, equal CPURate %d", opts.CPUQuota, cpuRate)
		nodesInfo, total, err = c.scheduler.SelectMemoryNodes(nodesInfo, cpuRate, opts.Memory) // 还是以 Bytes 作单位， 不转换了
	case scheduler.CPU_PRIOR:
		nodesInfo, nodeCPUPlans, total, err = c.scheduler.SelectCPUNodes(nodesInfo, opts.CPUQuota, opts.Memory)
	default:
		return nil, nil, fmt.Errorf("Pod type not support yet")
	}
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("[allocResource] Get capacity %v", nodesInfo)

	switch opts.DeployMethod {
	case cluster.DeployAuto:
		nodesInfo, err = c.scheduler.CommonDivision(nodesInfo, opts.Count, total)
	case cluster.DeployEach:
		nodesInfo, err = c.scheduler.EachDivision(nodesInfo, opts.Count, total)
	case cluster.DeployFill:
	default:
		return nil, nil, fmt.Errorf("Deploy method not support yet")
	}
	if err != nil {
		return nil, nil, err
	}
	log.Debugf("[allocResource] Get deploy plan %v", nodesInfo)

	// 资源处理
	switch podType {
	case scheduler.MEMORY_PRIOR:
		// 并发扣除所需资源
		wg := sync.WaitGroup{}
		for _, nodeInfo := range nodesInfo {
			if nodeInfo.Deploy > 0 {
				wg.Add(1)
				go func(nodeInfo types.NodeInfo) {
					defer wg.Done()
					memoryTotal := opts.Memory * int64(nodeInfo.Deploy)
					c.store.UpdateNodeResource(ctx, opts.Podname, nodeInfo.Name, types.CPUMap{}, memoryTotal, "-")
				}(nodeInfo)
			}
		}
		wg.Wait()
		return nil, nodesInfo, nil
	case scheduler.CPU_PRIOR:
		result, changed := c.scheduler.MakeCPUPlan(nodesInfo, nodeCPUPlans)
		// if quota is set to 0
		// then no cpu is required
		if opts.CPUQuota > 0 {
			// cpus changeded
			// update data to etcd
			// `SelectCPUNodes` reduces count in cpumap
			for _, node := range nodes {
				r, ok := changed[node.Name]
				// 不在changed里说明没有变化
				if ok {
					node.CPU = r
					node.MemCap = node.MemCap - opts.Memory*int64(len(result[node.Name]))
					if err := c.store.UpdateNode(ctx, node); err != nil {
						return nil, nil, err
					}
				}
			}
		}
		return result, nil, nil
	}
	return nil, nil, fmt.Errorf("Resource alloc failed")
}

func (c *Calcium) getCPUAndMem(ctx context.Context, podname, nodename string, labels map[string]string) (map[string]types.CPUAndMem, []*types.Node, error) {
	result := make(map[string]types.CPUAndMem)
	var nodes []*types.Node
	var err error
	if nodename == "" {
		nodes, err = c.ListPodNodes(ctx, podname, false)
		if err != nil {
			return result, nil, err
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
			return result, nil, err
		}
		nodes = append(nodes, n)
	}

	if len(nodes) == 0 {
		err := fmt.Errorf("No available nodes")
		return result, nil, err
	}

	result = makeCPUAndMem(nodes)
	go stats.Client.SendMemCap(result)
	return result, nodes, nil
}
