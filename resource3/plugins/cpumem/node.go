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
	}

	// if NUMA is set but NUMAMemory is not set
	// then divide memory equally according to the number of numa nodes
	if len(req.NUMA) > 0 && (req.NUMAMemory == nil || len(req.NUMAMemory) == 0) {
		averageMemory := req.Memory / int64(len(req.NUMA))
		nodeResourceInfo.Capacity.NUMAMemory = cpumemtypes.NUMAMemory{}
		for _, ID := range req.NUMA {
			nodeResourceInfo.Capacity.NUMAMemory[ID] = averageMemory
		}
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
func (p Plugin) GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource *plugintypes.WorkloadResourceRequest) (*plugintypes.GetNodesDeployCapacityResponse, error) {
	logger := log.WithFunc("resource.cpumem.GetNodesDeployCapacity")
	req := &cpumemtypes.WorkloadResourceRequest{}
	if err := req.Parse(resource); err != nil {
		return nil, err
	}

	if err := req.Validate(); err != nil {
		logger.Errorf(ctx, err, "invalid resource opts %+v", req)
	}

	nodesDeployCapacityMap := map[string]*plugintypes.NodeDeployCapacity{}
	total := 0

	nodesResourceInfos, err := p.doGetNodesResourceInfo(ctx, nodenames)
	if err != nil {
		return nil, err
	}

	for nodename, nodeResourceInfo := range nodesResourceInfos {
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
	logger := log.WithFunc("resource.cpumem.SetNodeResourceCapacity").WithField("node", "nodename")
	req, nodeResource, _, nodeResourceInfo, err := p.parseNodeResourceInfos(ctx, nodename, resource, resourceRequest, nil)
	if err != nil {
		return nil, err
	}
	origin := nodeResourceInfo.Capacity
	before := origin.DeepCopy()

	if !delta && req != nil {
		req.LoadFromOrigin(origin, resourceRequest)
	}
	nodeResourceInfo.Capacity = p.calculateNodeResource(req, nodeResource, origin, nil, delta, incr)

	// add new cpu
	for cpu := range nodeResourceInfo.Capacity.CPUMap {
		if _, ok := nodeResourceInfo.Usage.CPUMap[cpu]; !ok {
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
		"before": before,
		"after":  nodeResourceInfo.Capacity,
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
func (p Plugin) SetNodeResourceInfo(ctx context.Context, nodename string, capacity *plugintypes.NodeResource, usage *plugintypes.NodeResource) (*plugintypes.SetNodeResourceInfoResponse, error) {
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
	logger := log.WithFunc("resource.cpumem.SetNodeResourceUsage").WithField("node", "nodename")
	req, nodeResource, wrksResource, nodeResourceInfo, err := p.parseNodeResourceInfos(ctx, nodename, resource, resourceRequest, workloadsResource)
	if err != nil {
		return nil, err
	}
	origin := nodeResourceInfo.Usage
	before := origin.DeepCopy()

	nodeResourceInfo.Usage = p.calculateNodeResource(req, nodeResource, origin, wrksResource, delta, incr)

	if err := p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		logger.Errorf(ctx, err, "node resource info %+v", litter.Sdump(nodeResourceInfo))
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
	var mostIdleNode string
	var minIdle = math.MaxFloat64

	for _, nodename := range nodenames {
		resourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
		if err != nil {
			log.WithFunc("resource.cpumem.GetMostIdleNode").WithField("node", nodename).Error(ctx, err)
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
	logger := log.WithFunc("resource.cpumem.getNodeResourceInfo").WithField("node", nodename)
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

	for cpu := range nodeResourceInfo.Usage.CPUMap {
		if actuallyWorkloadsUsage.CPUMap[cpu] != nodeResourceInfo.Usage.CPUMap[cpu] {
			diffs = append(diffs, fmt.Sprintf("node.CPUMap[%+v] != sum(workload.CPUMap[%+v]): %+v != %+v", cpu, cpu, nodeResourceInfo.Usage.CPUMap[cpu], actuallyWorkloadsUsage.CPUMap[cpu]))
		}
	}

	for numaNodeID := range nodeResourceInfo.Usage.NUMAMemory {
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
	resp, err := p.doGetNodesResourceInfo(ctx, []string{nodename})
	if err != nil {
		return nil, err
	}
	return resp[nodename], err
}

func (p Plugin) doGetNodesResourceInfo(ctx context.Context, nodenames []string) (map[string]*cpumemtypes.NodeResourceInfo, error) {
	keys := []string{}
	for _, nodename := range nodenames {
		keys = append(keys, fmt.Sprintf(nodeResourceInfoKey, nodename))
	}
	resps, err := p.store.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	result := map[string]*cpumemtypes.NodeResourceInfo{}

	for _, resp := range resps {
		r := &cpumemtypes.NodeResourceInfo{}
		if err := json.Unmarshal(resp.Value, r); err != nil {
			return nil, err
		}
		result[utils.Tail(string(resp.Key))] = r
	}
	return result, nil
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

func (p Plugin) doGetNodeDeployCapacity(nodeResourceInfo *cpumemtypes.NodeResourceInfo, req *cpumemtypes.WorkloadResourceRequest) *plugintypes.NodeDeployCapacity {
	availableResource := nodeResourceInfo.GetAvailableResource()

	capacityInfo := &plugintypes.NodeDeployCapacity{
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
	capacityInfo.Weight = 100 // cpu-bind above all
	return capacityInfo
}

// calculateNodeResource priority: node resource request > node resource > workload resource args list
func (p Plugin) calculateNodeResource(req *cpumemtypes.NodeResourceRequest, nodeResource *cpumemtypes.NodeResource, origin *cpumemtypes.NodeResource, workloadsResource []*cpumemtypes.WorkloadResource, delta bool, incr bool) *cpumemtypes.NodeResource {
	var resp *cpumemtypes.NodeResource
	if origin == nil || !delta { // no delta means node resource rewrite with whole new data
		resp = (&cpumemtypes.NodeResource{}).DeepCopy() // init nil pointer!
		// 这个接口最诡异的在于，如果 delta 为 false，意味着是全量写入
		// 但这时候 incr 为 false 的话
		// 实际上是 set 进了负值，所以这里 incr 应该强制为 true
		incr = true
	} else {
		resp = origin.DeepCopy()
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

func (p Plugin) parseNodeResourceInfos(
	ctx context.Context, nodename string,
	resource *plugintypes.NodeResource,
	resourceRequest *plugintypes.NodeResourceRequest,
	workloadsResource []*plugintypes.WorkloadResource,
) (
	*cpumemtypes.NodeResourceRequest,
	*cpumemtypes.NodeResource,
	[]*cpumemtypes.WorkloadResource,
	*cpumemtypes.NodeResourceInfo,
	error,
) {
	var req *cpumemtypes.NodeResourceRequest
	var nodeResource *cpumemtypes.NodeResource
	wrksResource := []*cpumemtypes.WorkloadResource{}

	if resourceRequest != nil {
		req = &cpumemtypes.NodeResourceRequest{}
		if err := req.Parse(p.config, resourceRequest); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	if resource != nil {
		nodeResource = &cpumemtypes.NodeResource{}
		if err := nodeResource.Parse(resource); err != nil {
			return nil, nil, nil, nil, err
		}
	}

	for _, workloadResource := range workloadsResource {
		wrkResource := &cpumemtypes.WorkloadResource{}
		if err := wrkResource.Parse(workloadResource); err != nil {
			return nil, nil, nil, nil, err
		}
		wrksResource = append(wrksResource, wrkResource)
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		return nil, nil, nil, nil, err
	}
	return req, nodeResource, wrksResource, nodeResourceInfo, nil
}
