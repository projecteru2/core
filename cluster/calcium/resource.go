package calcium

import (
	"context"
	"fmt"
	"sort"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/scheduler/complex"
	"github.com/projecteru2/core/stats"
	"github.com/projecteru2/core/types"
)

func (c *Calcium) allocMemoryPodResource(ctx context.Context, opts *types.DeployOptions) ([]types.NodeInfo, error) {
	lock, err := c.Lock(ctx, opts.Podname, c.config.LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock(ctx)

	cpuandmem, _, err := c.getCPUAndMem(ctx, opts.Podname, opts.Nodename, opts.NodeLabels)
	if err != nil {
		return nil, err
	}

	nodesInfo := getNodesInfo(cpuandmem)

	// Load deploy status
	nodesInfo, err = c.store.MakeDeployStatus(ctx, opts, nodesInfo)
	if err != nil {
		return nil, err
	}

	// 每台机器都允许部署所需容量容器
	if opts.RawResource {
		for i := range nodesInfo {
			nodesInfo[i].Capacity = opts.Count
		}
		return complexscheduler.CommunismDivisionPlan(nodesInfo, opts.Count, opts.Count*len(nodesInfo))
	}

	cpuRate := int64(opts.CPUQuota * float64(CpuPeriodBase))
	log.Debugf("[allocMemoryPodResource] Input opts.CPUQuota: %f, equal CPURate %d", opts.CPUQuota, cpuRate)
	sort.Slice(nodesInfo, func(i, j int) bool { return nodesInfo[i].MemCap < nodesInfo[j].MemCap })
	nodesInfo, err = c.scheduler.SelectMemoryNodes(nodesInfo, cpuRate, opts.Memory, opts.Count) // 还是以 Bytes 作单位， 不转换了
	if err != nil {
		return nil, err
	}

	// 并发扣除所需资源
	wg := sync.WaitGroup{}
	for _, nodeInfo := range nodesInfo {
		if nodeInfo.Deploy <= 0 {
			continue
		}
		wg.Add(1)
		go func(nodeInfo types.NodeInfo) {
			defer wg.Done()
			memoryTotal := opts.Memory * int64(nodeInfo.Deploy)
			c.store.UpdateNodeMem(ctx, opts.Podname, nodeInfo.Name, memoryTotal, "-")
		}(nodeInfo)
	}
	wg.Wait()
	return nodesInfo, nil
}

func (c *Calcium) allocCPUPodResource(ctx context.Context, opts *types.DeployOptions) (map[string][]types.CPUMap, error) {
	lock, err := c.Lock(ctx, opts.Podname, c.config.LockTimeout)
	if err != nil {
		return nil, err
	}
	defer lock.Unlock(ctx)

	cpuandmem, nodes, err := c.getCPUAndMem(ctx, opts.Podname, opts.Nodename, opts.NodeLabels)
	if err != nil {
		return nil, err
	}
	nodesInfo := getNodesInfo(cpuandmem)

	// Load deploy status
	nodesInfo, err = c.store.MakeDeployStatus(ctx, opts, nodesInfo)
	if err != nil {
		return nil, err
	}

	result, changed, err := c.scheduler.SelectCPUNodes(nodesInfo, opts.CPUQuota, opts.Count)
	log.Debugf("[allocCPUPodResource] Result: %v, Changed: %v", result, changed)
	if err != nil {
		return result, err
	}

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
				// ignore error
				c.store.UpdateNode(ctx, node)
			}
		}
	}

	return result, err
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
