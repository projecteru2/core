package cpumem

import (
	"context"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins/cpumem/schedule"
	cpumemtypes "github.com/projecteru2/core/resource/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	coretypes "github.com/projecteru2/core/types"
)

// CalculateDeploy .
func (p Plugin) CalculateDeploy(ctx context.Context, nodename string, deployCount int, resourceRequest plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateDeployResponse, error) {
	logger := log.WithFunc("resource.cpumem.CalculateDeploy").WithField("node", nodename)
	req := &cpumemtypes.WorkloadResourceRequest{}
	if err := req.Parse(resourceRequest); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		logger.Errorf(ctx, err, "invalid resource opts %+v", req)
		return nil, err
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		logger.WithField("node", nodename).Error(ctx, err)
		return nil, err
	}

	var enginesParams []*cpumemtypes.EngineParams
	var workloadsResource []*cpumemtypes.WorkloadResource

	if !req.CPUBind {
		enginesParams, workloadsResource, err = p.doAllocByMemory(nodeResourceInfo, deployCount, req)
	} else {
		enginesParams, workloadsResource, err = p.doAllocByCPU(nodeResourceInfo, deployCount, req)
	}
	if err != nil {
		return nil, err
	}

	resp := &plugintypes.CalculateDeployResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engines_params":     enginesParams,
		"workloads_resource": workloadsResource,
	}, resp)
}

// CalculateRealloc .
func (p Plugin) CalculateRealloc(ctx context.Context, nodename string, resource plugintypes.WorkloadResource, resourceRequest plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateReallocResponse, error) {
	req := &cpumemtypes.WorkloadResourceRequest{}
	if err := req.Parse(resourceRequest); err != nil {
		return nil, err
	}
	originResource := &cpumemtypes.WorkloadResource{}
	if err := originResource.Parse(resource); err != nil {
		return nil, err
	}

	if req.KeepCPUBind {
		req.CPUBind = len(originResource.CPUMap) > 0
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		log.WithFunc("resource.cpumem.CalculateRealloc").WithField("node", nodename).Error(ctx, err, "failed to get resource info of node")
		return nil, err
	}

	// put resources back into the resource pool
	nodeResourceInfo.Usage.Sub(&cpumemtypes.NodeResource{
		CPU:        originResource.CPURequest,
		CPUMap:     originResource.CPUMap,
		Memory:     originResource.MemoryRequest,
		NUMAMemory: originResource.NUMAMemory,
	})

	newReq := &cpumemtypes.WorkloadResourceRequest{
		CPUBind:    req.CPUBind,
		CPURequest: req.CPURequest + originResource.CPURequest,
		CPULimit:   req.CPULimit + originResource.CPULimit,
		MemRequest: req.MemRequest + originResource.MemoryRequest,
		MemLimit:   req.MemLimit + originResource.MemoryLimit,
	}

	if err = newReq.Validate(); err != nil {
		return nil, err
	}

	// if cpu was specified before, try to ensure cpu affinity
	var cpuMap cpumemtypes.CPUMap
	var numaNodeID string
	var numaMemory cpumemtypes.NUMAMemory

	if req.CPUBind {
		cpuPlans := schedule.GetCPUPlans(nodeResourceInfo, originResource.CPUMap, p.config.Scheduler.ShareBase, p.config.Scheduler.MaxShare, newReq)
		if len(cpuPlans) == 0 {
			return nil, coretypes.ErrInsufficientResource
		}

		cpuPlan := cpuPlans[0]
		cpuMap = cpuPlan.CPUMap
		numaNodeID = cpuPlan.NUMANode
		if len(numaNodeID) > 0 {
			numaMemory = cpumemtypes.NUMAMemory{numaNodeID: newReq.MemRequest}
		}
	} else if _, _, err = p.doAllocByMemory(nodeResourceInfo, 1, newReq); err != nil {
		return nil, err
	}

	engineParams := &cpumemtypes.EngineParams{
		CPU:      newReq.CPULimit,
		CPUMap:   cpuMap,
		NUMANode: numaNodeID,
		Memory:   newReq.MemLimit,
	}

	newResource := &cpumemtypes.WorkloadResource{
		CPURequest:    newReq.CPURequest,
		CPULimit:      newReq.CPULimit,
		MemoryRequest: newReq.MemRequest,
		MemoryLimit:   newReq.MemLimit,
		CPUMap:        cpuMap,
		NUMAMemory:    numaMemory,
		NUMANode:      numaNodeID,
	}

	deltaWorkloadResource := newResource.DeepCopy()
	deltaWorkloadResource.Sub(originResource)

	resp := &plugintypes.CalculateReallocResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_params":     engineParams,
		"delta_resource":    deltaWorkloadResource,
		"workload_resource": newResource,
	}, resp)
}

// CalculateRemap .
func (p Plugin) CalculateRemap(ctx context.Context, nodename string, workloadsResource map[string]plugintypes.WorkloadResource) (*plugintypes.CalculateRemapResponse, error) {
	resp := &plugintypes.CalculateRemapResponse{}
	engineParamsMap := map[string]*cpumemtypes.EngineParams{}
	if len(workloadsResource) == 0 {
		return resp, mapstructure.Decode(map[string]interface{}{
			"engine_params_map": engineParamsMap,
		}, resp)
	}

	workloadResourceMap := map[string]*cpumemtypes.WorkloadResource{}
	for ID, workload := range workloadsResource {
		workloadResource := &cpumemtypes.WorkloadResource{}
		if err := workloadResource.Parse(workload); err != nil {
			return nil, err
		}
		workloadResourceMap[ID] = workloadResource
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		log.WithFunc("resource.cpumem.CalculateRemap").WithField("node", nodename).Error(ctx, err)
		return nil, err
	}

	availableNodeResource := nodeResourceInfo.GetAvailableResource()

	shareCPUMap := cpumemtypes.CPUMap{}
	for cpu, pieces := range availableNodeResource.CPUMap {
		if pieces >= p.config.Scheduler.ShareBase {
			shareCPUMap[cpu] = p.config.Scheduler.ShareBase
		}
	}

	if len(shareCPUMap) == 0 {
		for cpu := range nodeResourceInfo.Capacity.CPUMap {
			shareCPUMap[cpu] = p.config.Scheduler.ShareBase
		}
	}

	for ID, workloadResource := range workloadResourceMap {
		// only process workloads without cpu binding
		if len(workloadResource.CPUMap) == 0 {
			engineParamsMap[ID] = &cpumemtypes.EngineParams{
				CPU:      workloadResource.CPULimit,
				CPUMap:   shareCPUMap,
				NUMANode: workloadResource.NUMANode,
				Memory:   workloadResource.MemoryLimit,
				Remap:    true,
			}
		}
	}

	return resp, mapstructure.Decode(map[string]interface{}{
		"engine_params_map": engineParamsMap,
	}, resp)
}

func (p Plugin) doAllocByMemory(resourceInfo *cpumemtypes.NodeResourceInfo, deployCount int, req *cpumemtypes.WorkloadResourceRequest) ([]*cpumemtypes.EngineParams, []*cpumemtypes.WorkloadResource, error) {
	if req.CPURequest > float64(len(resourceInfo.Capacity.CPUMap)) {
		return nil, nil, errors.Wrap(coretypes.ErrInsufficientCapacity, "cpu")
	}

	availableResource := resourceInfo.GetAvailableResource()
	if req.MemRequest > 0 && availableResource.Memory/req.MemRequest < int64(deployCount) {
		return nil, nil, errors.Wrap(coretypes.ErrInsufficientCapacity, "memory")
	}

	enginesParams := []*cpumemtypes.EngineParams{}
	workloadsResource := []*cpumemtypes.WorkloadResource{}

	engineParams := &cpumemtypes.EngineParams{
		CPU:    req.CPULimit,
		Memory: req.MemLimit,
	}
	workloadResource := &cpumemtypes.WorkloadResource{
		CPURequest:    req.CPURequest,
		CPULimit:      req.CPULimit,
		MemoryRequest: req.MemRequest,
		MemoryLimit:   req.MemLimit,
	}

	for len(enginesParams) < deployCount {
		enginesParams = append(enginesParams, engineParams)
		workloadsResource = append(workloadsResource, workloadResource)
	}
	return enginesParams, workloadsResource, nil
}

func (p Plugin) doAllocByCPU(resourceInfo *cpumemtypes.NodeResourceInfo, deployCount int, req *cpumemtypes.WorkloadResourceRequest) ([]*cpumemtypes.EngineParams, []*cpumemtypes.WorkloadResource, error) {
	cpuPlans := schedule.GetCPUPlans(resourceInfo, nil, p.config.Scheduler.ShareBase, p.config.Scheduler.MaxShare, req)
	if len(cpuPlans) < deployCount {
		return nil, nil, errors.Wrap(coretypes.ErrInsufficientCapacity, "cpu")
	}

	cpuPlans = cpuPlans[:deployCount]
	enginesParams := []*cpumemtypes.EngineParams{}
	workloadsResource := []*cpumemtypes.WorkloadResource{}

	for _, cpuPlan := range cpuPlans {
		enginesParams = append(enginesParams, &cpumemtypes.EngineParams{
			CPU:      req.CPULimit,
			CPUMap:   cpuPlan.CPUMap,
			NUMANode: cpuPlan.NUMANode,
			Memory:   req.MemLimit,
		})

		workloadResource := &cpumemtypes.WorkloadResource{
			CPURequest:    req.CPURequest,
			CPULimit:      req.CPULimit,
			MemoryRequest: req.MemRequest,
			MemoryLimit:   req.MemLimit,
			CPUMap:        cpuPlan.CPUMap,
			NUMANode:      cpuPlan.NUMANode,
		}
		if len(workloadResource.NUMANode) > 0 {
			workloadResource.NUMAMemory = cpumemtypes.NUMAMemory{workloadResource.NUMANode: workloadResource.MemoryRequest}
		}

		workloadsResource = append(workloadsResource, workloadResource)
	}

	return enginesParams, workloadsResource, nil
}
