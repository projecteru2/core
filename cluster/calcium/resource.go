package calcium

import (
	"context"
	"fmt"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/resources"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/strategy"
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
		VolumePercents:  map[string]float64{},
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
		r.VolumePercents[node.Name] = nodeDetail.VolumePercent
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
			cpus = utils.Round(cpus + container.QuotaRequest)
			memory += container.MemoryRequest
			storage += container.StorageRequest
			cpumap.Add(container.CPURequest)
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
			return c.store.UpdateNodes(ctx, n)
		},
		nil,
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doAllocResource(ctx context.Context, nodeMap map[string]*types.Node, opts *types.DeployOptions) (map[types.ResourceType]resourcetypes.ResourcePlans, map[string]*types.DeployInfo, error) {
	if len(nodeMap) == 0 {
		return nil, nil, errors.WithStack(types.ErrInsufficientNodes)
	}

	apps, err := resources.NewResourceRequirements(opts.RawResourceOptions)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}

	// select available nodes
	planMap, total, scheduleTypes, err := resources.SelectNodes(apps, nodeMap)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	log.Errorf("[Calcium.doAllocResource] planMap: %+v, total: %v, type: %+v", planMap, total, scheduleTypes)

	// deploy strategy
	strategyInfos := strategy.NewStrategyInfos(apps, nodeMap, planMap)
	if err := c.store.MakeDeployStatus(ctx, opts, strategyInfos); err != nil {
		return nil, nil, errors.WithStack(err)
	}
	deployMap, err := strategy.Deploy(opts, strategyInfos, total, scheduleTypes)
	if err != nil {
		return nil, nil, errors.WithStack(err)
	}
	log.Debugf("[Calium.doAllocResource] deployMap: %+v", deployMap)

	return planMap, deployMap, nil
}
