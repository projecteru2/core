package cpumem

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"strconv"

	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource3/plugins/cpumem/schedule"
	cpumemtypes "github.com/projecteru2/core/resource3/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/sanity-io/litter"
)

// AddNode .
func (p Plugin) AddNode(ctx context.Context, nodename string, resource *plugintypes.NodeResourceRequest, info *enginetypes.Info) (*plugintypes.AddNodeResponse, error) {
	// try to get the node resource
	var err error
	if _, err = p.doGetNodeResourceInfo(ctx, nodename); err == nil {
		return nil, coretypes.ErrNodeExists
	}

	if !errors.Is(err, coretypes.ErrInvaildCount) {
		log.WithFunc("resource.cpumem.AddNode").WithField("node", nodename).Error(ctx, err, "failed to get resource info of node")
		return nil, err
	}

	req := &cpumemtypes.NodeResourceRequest{}
	if err := req.Parse(p.config, resource); err != nil {
		return nil, err
	}

	if info != nil {
		if len(req.CPUMap) == 0 {
			req.CPUMap = cpumemtypes.CPUMap{}
			for i := 0; i < info.NCPU; i++ {
				req.CPUMap[strconv.Itoa(i)] = p.config.Scheduler.ShareBase
			}
		}

		if req.Memory == 0 {
			req.Memory = info.MemTotal * rate / 10 // use 80% of real memory
		}
	}

	nodeResourceInfo := &cpumemtypes.NodeResourceInfo{
		Capacity: &cpumemtypes.NodeResource{
			CPU:        float64(len(req.CPUMap)),
			CPUMap:     req.CPUMap,
			Memory:     req.Memory,
			NUMAMemory: req.NUMAMemory,
			NUMA:       req.NUMA,
		},
		Usage: &cpumemtypes.NodeResource{
			CPU:        0,
			CPUMap:     cpumemtypes.CPUMap{},
			Memory:     0,
			NUMAMemory: cpumemtypes.NUMAMemory{},
		},
	}

	// if NUMA is set but NUMAMemory is not set
	// then divide memory equally according to the number of numa nodes
	if len(req.NUMA) > 0 && req.NUMAMemory == nil {
		averageMemory := req.Memory / int64(len(req.NUMA))
		nodeResourceInfo.Capacity.NUMAMemory = cpumemtypes.NUMAMemory{}
		for _, ID := range req.NUMA {
			nodeResourceInfo.Capacity.NUMAMemory[ID] = averageMemory
		}
	}

	for cpu := range req.CPUMap {
		nodeResourceInfo.Usage.CPUMap[cpu] = 0
	}

	for ID := range req.NUMA {
		nodeResourceInfo.Usage.NUMAMemory[ID] = 0
	}

	if err = p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		return nil, err
	}

	resp := &plugintypes.AddNodeResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"capacity": nodeResourceInfo.Capacity,
		"usage":    nodeResourceInfo.Usage,
	}, resp)
}

// RemoveNode .
func (p Plugin) RemoveNode(ctx context.Context, nodename string) (*plugintypes.RemoveNodeResponse, error) {
	var err error
	if _, err = p.store.Delete(ctx, fmt.Sprintf(nodeResourceInfoKey, nodename)); err != nil {
		log.WithFunc("resource.cpumem.RemoveNode").WithField("node", nodename).Error(ctx, err, "faield to delete node")
	}
	return &plugintypes.RemoveNodeResponse{}, err
}

// GetNodesDeployCapacity returns available nodes and total capacity
func (p Plugin) GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource *plugintypes.WorkloadResource) (*plugintypes.GetNodesDeployCapacityResponse, error) {
	logger := log.WithFunc("resource.cpumem.GetNodesDeployCapacity")
	req := &cpumemtypes.WorkloadResourceRequest{}
	if err := req.Parse(resource); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		logger.Errorf(ctx, err, "invalid resource opts %+v", req)
	}

	nodesDeployCapacityMap := map[string]*cpumemtypes.NodeDeployCapacity{}
	total := 0
	for _, nodename := range nodenames {
		nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
		if err != nil {
			logger.WithField("node", nodename).Error(ctx, err)
			return nil, err
		}
		nodeDeployCapacity := p.doGetNodeDeployCapacity(nodeResourceInfo, req)
		if nodeDeployCapacity.Capacity > 0 {
			nodesDeployCapacityMap[nodename] = nodeDeployCapacity
			if total == math.MaxInt || nodeDeployCapacity.Capacity == math.MaxInt {
				total = math.MaxInt
			} else {
				total += nodeDeployCapacity.Capacity
			}
		}
	}

	resp := &plugintypes.GetNodesDeployCapacityResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"nodes_deploy_capacity_map": nodesDeployCapacityMap,
		"total":                     total,
	}, resp)
}

// SetNodeResourceCapacity sets the amount of total resource info
func (p Plugin) SetNodeResourceCapacity(ctx context.Context, nodename string, resource *plugintypes.NodeResource, resourceRequest *plugintypes.NodeResourceRequest, delta bool, incr bool) (*plugintypes.SetNodeResourceCapacityResponse, error) {
	var req *cpumemtypes.NodeResourceRequest
	var nodeResource *cpumemtypes.NodeResource
	logger := log.WithFunc("resource.cpumem.SetNodeResourceCapacity").WithField("node", "nodename")

	if resourceRequest != nil {
		req = &cpumemtypes.NodeResourceRequest{}
		if err := req.Parse(p.config, resourceRequest); err != nil {
			return nil, err
		}
	}

	if resource != nil {
		nodeResource = &cpumemtypes.NodeResource{}
		if err := nodeResource.Parse(resource); err != nil {
			return nil, err
		}
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	origin := nodeResourceInfo.Capacity.DeepCopy()
	if !delta && req != nil {
		req.LoadFromNodeResource(nodeResourceInfo.Capacity, resourceRequest)
	}

	nodeResourceInfo.Capacity = p.calculateNodeResource(req, nodeResource, nodeResourceInfo.Capacity, nil, delta, incr)

	// add new cpu
	for cpu := range nodeResourceInfo.Capacity.CPUMap {
		_, ok := nodeResourceInfo.Usage.CPUMap[cpu]
		if !ok {
			nodeResourceInfo.Usage.CPUMap[cpu] = 0
		}
	}
	// delete cpus with no pieces
	nodeResourceInfo.RemoveEmptyCores()

	if err := p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		logger.Errorf(ctx, err, "node resource info %+v", litter.Sdump(nodeResourceInfo))
		return nil, err
	}

	resp := &plugintypes.SetNodeResourceCapacityResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"Origin": origin,
		"Target": nodeResourceInfo.Capacity,
	}, resp)
}

// GetNodeResourceInfo .
func (p Plugin) GetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error) {
	nodeResourceInfo, _, diffs, err := p.getNodeResourceInfo(ctx, nodename, workloadsResource)
	if err != nil {
		return nil, err
	}

	resp := &plugintypes.GetNodeResourceInfoResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"capacity": nodeResourceInfo.Capacity,
		"usage":    nodeResourceInfo.Usage,
		"diffs":    diffs,
	}, resp)
}

// SetNodeResourceInfo .
func (p Plugin) SetNodeResourceInfo(ctx context.Context, nodename string, capacity *plugintypes.NodeResourceRequest, usage *plugintypes.NodeResourceRequest) (*plugintypes.SetNodeResourceInfoResponse, error) {
	capacityResource := &cpumemtypes.NodeResource{}
	usageResource := &cpumemtypes.NodeResource{}
	if err := capacityResource.Parse(capacity); err != nil {
		return nil, err
	}
	if err := usageResource.Parse(usage); err != nil {
		return nil, err
	}
	resourceInfo := &cpumemtypes.NodeResourceInfo{
		Capacity: capacityResource,
		Usage:    usageResource,
	}

	return &plugintypes.SetNodeResourceInfoResponse{}, p.doSetNodeResourceInfo(ctx, nodename, resourceInfo)
}

// SetNodeResourceUsage .
func (p Plugin) SetNodeResourceUsage(ctx context.Context, nodename string, resource *plugintypes.NodeResource, resourceRequest *plugintypes.NodeResourceRequest, workloadsResource []*plugintypes.WorkloadResource, delta bool, incr bool) (*plugintypes.SetNodeResourceUsageResponse, error) {
	var req *cpumemtypes.NodeResourceRequest
	var nodeResource *cpumemtypes.NodeResource

	if resourceRequest != nil {
		req = &cpumemtypes.NodeResourceRequest{}
		if err := req.Parse(p.config, resourceRequest); err != nil {
			return nil, err
		}
	}

	if resource != nil {
		nodeResource = &cpumemtypes.NodeResource{}
		if err := nodeResource.Parse(resource); err != nil {
			return nil, err
		}
	}

	wrksResource := []*cpumemtypes.WorkloadResource{}
	for _, workloadResource := range workloadsResource {
		wrkResource := &cpumemtypes.WorkloadResource{}
		if err := wrkResource.Parse(workloadResource); err != nil {
			return nil, err
		}
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		log.WithFunc("resource.cpumem.SetNodeResourceUsage").WithField("node", nodename).Error(ctx, err)
		return nil, err
	}

	before := nodeResourceInfo.Usage.DeepCopy()
	nodeResourceInfo.Usage = p.calculateNodeResource(req, nodeResource, nodeResourceInfo.Capacity, wrksResource, delta, incr)

	if err := p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		return nil, err
	}

	resp := &plugintypes.SetNodeResourceUsageResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"before": before,
		"after":  nodeResourceInfo,
	}, resp)
}

// GetMostIdleNode .
func (p Plugin) GetMostIdleNode(ctx context.Context, nodenames []string) (*plugintypes.GetMostIdleNodeResponse, error) {
	const priority = 100
	var mostIdleNode string
	var minIdle = math.MaxFloat64

	for _, nodename := range nodenames {
		resourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
		if err != nil {
			log.WithFunc("resource.cpumem.GetMostIdleNode").Error(ctx, err, "failed to get node resource info")
			return nil, err
		}
		idle := float64(resourceInfo.Usage.CPUMap.TotalPieces()) / float64(resourceInfo.Capacity.CPUMap.TotalPieces())
		idle += float64(resourceInfo.Usage.Memory) / float64(resourceInfo.Capacity.Memory)

		if idle < minIdle {
			mostIdleNode = nodename
			minIdle = idle
		}
	}

	resp := &plugintypes.GetMostIdleNodeResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"nodename": mostIdleNode,
		"priority": priority,
	}, resp)
}

// FixNodeResource .
func (p Plugin) FixNodeResource(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error) {
	nodeResourceInfo, actuallyWorkloadsUsage, diffs, err := p.getNodeResourceInfo(ctx, nodename, workloadsResource)
	if err != nil {
		return nil, err
	}

	if len(diffs) != 0 {
		nodeResourceInfo.Usage = &cpumemtypes.NodeResource{
			CPU:        actuallyWorkloadsUsage.CPURequest,
			CPUMap:     actuallyWorkloadsUsage.CPUMap,
			Memory:     actuallyWorkloadsUsage.MemoryRequest,
			NUMAMemory: actuallyWorkloadsUsage.NUMAMemory,
		}
		if err = p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
			log.WithFunc("resource.cpumem.FixNodeResource").Error(ctx, err)
			diffs = append(diffs, err.Error())
		}
	}

	resp := &plugintypes.GetNodeResourceInfoResponse{}
	return resp, mapstructure.Decode(map[string]interface{}{
		"capacity": nodeResourceInfo.Capacity,
		"usage":    nodeResourceInfo.Usage,
		"diffs":    diffs,
	}, resp)
}

func (p Plugin) getNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*cpumemtypes.NodeResourceInfo, *cpumemtypes.WorkloadResource, []string, error) {
	logger := log.WithFunc("resource.cpumem.GetNodeResourceInfo").WithField("node", nodename)
	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		logger.Error(ctx, err)
		return nil, nil, nil, err
	}

	actuallyWorkloadsUsage := &cpumemtypes.WorkloadResource{CPUMap: cpumemtypes.CPUMap{}, NUMAMemory: cpumemtypes.NUMAMemory{}}
	for _, workloadResource := range workloadsResource {
		workloadUsage := &cpumemtypes.WorkloadResource{}
		if err := workloadUsage.Parse(workloadResource); err != nil {
			logger.Error(ctx, err)
			return nil, nil, nil, err
		}
		actuallyWorkloadsUsage.Add(workloadUsage)
	}

	diffs := []string{}

	actuallyWorkloadsUsage.CPURequest = utils.Round(actuallyWorkloadsUsage.CPURequest)
	totalCPUUsage := utils.Round(nodeResourceInfo.Usage.CPU)
	if actuallyWorkloadsUsage.CPURequest != totalCPUUsage {
		diffs = append(diffs, fmt.Sprintf("node.CPUUsed != sum(workload.CPURequest): %.2f != %.2f", totalCPUUsage, actuallyWorkloadsUsage.CPURequest))
	}

	for cpu := range nodeResourceInfo.Capacity.CPUMap {
		if actuallyWorkloadsUsage.CPUMap[cpu] != nodeResourceInfo.Usage.CPUMap[cpu] {
			diffs = append(diffs, fmt.Sprintf("node.CPUMap[%+v] != sum(workload.CPUMap[%+v]): %+v != %+v", cpu, cpu, nodeResourceInfo.Usage.CPUMap[cpu], actuallyWorkloadsUsage.CPUMap[cpu]))
		}
	}

	for numaNodeID := range nodeResourceInfo.Capacity.NUMAMemory {
		if actuallyWorkloadsUsage.NUMAMemory[numaNodeID] != nodeResourceInfo.Usage.NUMAMemory[numaNodeID] {
			diffs = append(diffs, fmt.Sprintf("node.NUMAMemory[%+v] != sum(workload.NUMAMemory[%+v]: %+v != %+v)", numaNodeID, numaNodeID, nodeResourceInfo.Usage.NUMAMemory[numaNodeID], actuallyWorkloadsUsage.NUMAMemory[numaNodeID]))
		}
	}

	if nodeResourceInfo.Usage.Memory != actuallyWorkloadsUsage.MemoryRequest {
		diffs = append(diffs, fmt.Sprintf("node.MemoryUsed != sum(workload.MemoryRequest): %d != %d", nodeResourceInfo.Usage.Memory, actuallyWorkloadsUsage.MemoryRequest))
	}

	return nodeResourceInfo, actuallyWorkloadsUsage, diffs, nil
}

func (p Plugin) doGetNodeResourceInfo(ctx context.Context, nodename string) (*cpumemtypes.NodeResourceInfo, error) {
	resourceInfo := &cpumemtypes.NodeResourceInfo{}
	resp, err := p.store.GetOne(ctx, fmt.Sprintf(nodeResourceInfoKey, nodename))
	if err != nil {
		return nil, err
	}
	return resourceInfo, json.Unmarshal(resp.Value, resourceInfo)
}

func (p Plugin) doSetNodeResourceInfo(ctx context.Context, nodename string, resourceInfo *cpumemtypes.NodeResourceInfo) error {
	if err := resourceInfo.Validate(); err != nil {
		return err
	}

	data, err := json.Marshal(resourceInfo)
	if err != nil {
		return err
	}

	_, err = p.store.Put(ctx, fmt.Sprintf(nodeResourceInfoKey, nodename), string(data))
	return err
}

func (p Plugin) doGetNodeDeployCapacity(nodeResourceInfo *cpumemtypes.NodeResourceInfo, req *cpumemtypes.WorkloadResourceRequest) *cpumemtypes.NodeDeployCapacity {
	availableResource := nodeResourceInfo.GetAvailableResource()

	capacityInfo := &cpumemtypes.NodeDeployCapacity{
		Weight: 1, // TODO why 1?
	}

	// if cpu-bind is not required, then returns capacity by memory
	if !req.CPUBind {
		// check if cpu is enough
		if req.CPURequest > float64(len(nodeResourceInfo.Capacity.CPUMap)) {
			return capacityInfo
		}

		// calculate by memory request
		if req.MemRequest == 0 {
			capacityInfo.Capacity = math.MaxInt
			capacityInfo.Rate = 0
		} else {
			capacityInfo.Capacity = int(availableResource.Memory / req.MemRequest)
			capacityInfo.Rate = utils.AdvancedDivide(float64(req.MemRequest), float64(nodeResourceInfo.Capacity.Memory))
		}
		capacityInfo.Usage = utils.AdvancedDivide(float64(nodeResourceInfo.Usage.Memory), float64(nodeResourceInfo.Capacity.Memory))
		return capacityInfo
	}

	// if cpu-bind is required, then returns capacity by cpu scheduling
	cpuPlans := schedule.GetCPUPlans(nodeResourceInfo, nil, p.config.Scheduler.ShareBase, p.config.Scheduler.MaxShare, req)
	capacityInfo.Capacity = len(cpuPlans)
	capacityInfo.Usage = utils.AdvancedDivide(nodeResourceInfo.Usage.CPU, nodeResourceInfo.Capacity.CPU)
	capacityInfo.Rate = utils.AdvancedDivide(req.CPURequest, nodeResourceInfo.Capacity.CPU)
	capacityInfo.Weight = 100 // TODO why 100?
	return capacityInfo
}

// calculateNodeResource priority: node resource request > node resource > workload resource args list
func (p Plugin) calculateNodeResource(req *cpumemtypes.NodeResourceRequest, nodeResource *cpumemtypes.NodeResource, nodeCapacity *cpumemtypes.NodeResource, workloadsResource []*cpumemtypes.WorkloadResource, delta bool, incr bool) *cpumemtypes.NodeResource {
	var resp *cpumemtypes.NodeResource
	if nodeCapacity == nil || !delta { // no delta means node resource rewrite with whole new data
		resp = &cpumemtypes.NodeResource{}
	} else {
		resp = nodeCapacity.DeepCopy()
	}

	if req != nil {
		nodeResource = &cpumemtypes.NodeResource{
			CPU:        float64(len(req.CPUMap)),
			CPUMap:     req.CPUMap,
			Memory:     req.Memory,
			NUMAMemory: req.NUMAMemory,
			NUMA:       req.NUMA,
		}
	}

	if nodeResource != nil {
		if incr {
			resp.Add(nodeResource)
		} else {
			resp.Sub(nodeResource)
		}
		return resp
	}

	for _, workloadResource := range workloadsResource {
		nodeResource = &cpumemtypes.NodeResource{
			CPU:        workloadResource.CPURequest,
			CPUMap:     workloadResource.CPUMap,
			NUMAMemory: workloadResource.NUMAMemory,
			Memory:     workloadResource.MemoryRequest,
		}
		if incr {
			resp.Add(nodeResource)
		} else {
			resp.Sub(nodeResource)
		}
	}
	return resp
}