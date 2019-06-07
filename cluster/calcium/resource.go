package calcium

import (
	"context"
	"fmt"
	"sort"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// PodResource show pod resource usage
func (c *Calcium) PodResource(ctx context.Context, podname string) (*types.PodResource, error) {
	nodes, err := c.ListPodNodes(ctx, podname, true)
	if err != nil {
		return nil, err
	}
	r := &types.PodResource{
		Name:       podname,
		CPUPercent: map[string]float64{},
		MEMPercent: map[string]float64{},
		Diff:       map[string]bool{},
		Detail:     map[string]string{},
	}
	for _, node := range nodes {
		containers, err := c.ListNodeContainers(ctx, node.Name)
		if err != nil {
			return nil, err
		}
		cpus := 0.0
		memory := int64(0)
		cpumap := types.CPUMap{}
		for _, container := range containers {
			cpus = utils.Round(cpus + container.Quota)
			memory += container.Memory
			cpumap.Add(container.CPU)
		}
		r.CPUPercent[node.Name] = cpus / float64(len(node.InitCPU))
		r.MEMPercent[node.Name] = float64(memory) / float64(node.InitMemCap)
		r.Diff[node.Name] = true
		r.Detail[node.Name] = ""
		cpumap.Add(node.CPU)
		if cpus != node.CPUUsed {
			r.Diff[node.Name] = false
			r.Detail[node.Name] += fmt.Sprintf("cpus %f now %f ", node.CPUUsed, cpus)
		}
		for i, v := range cpumap {
			if node.InitCPU[i] != v {
				r.Diff[node.Name] = false
				r.Detail[node.Name] += fmt.Sprintf("cpu %s now %d ", i, v)
			}
		}
		if memory+node.MemCap != node.InitMemCap {
			r.Diff[node.Name] = false
			r.Detail[node.Name] += fmt.Sprintf("mem now %d ", node.InitMemCap-(memory+node.MemCap))
		}

	}
	return r, nil
}

func (c *Calcium) doAllocResource(ctx context.Context, opts *types.DeployOptions) ([]types.NodeInfo, error) {
	var err error
	var total int
	var nodesInfo []types.NodeInfo
	var nodeCPUPlans map[string][]types.CPUMap

	if err = c.withNodesLocked(ctx, opts.Podname, opts.Nodename, opts.NodeLabels, func(nodes map[string]*types.Node) error {
		nodesInfo = getNodesInfo(nodes, opts.CPUQuota, opts.Memory)
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
			quotaCost := opts.CPUQuota * float64(nodeInfo.Deploy)

			if _, ok := nodeCPUPlans[nodeInfo.Name]; ok {
				cpuList := nodeCPUPlans[nodeInfo.Name][:nodeInfo.Deploy]
				nodesInfo[i].CPUPlan = cpuList
				for _, cpu := range cpuList {
					cpuCost.Add(cpu)
				}
			}
			if err = c.store.UpdateNodeResource(ctx, nodes[nodeInfo.Name], cpuCost, quotaCost, memoryCost, store.ActionDecr); err != nil {
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
