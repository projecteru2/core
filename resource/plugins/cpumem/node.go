package cpumem

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"runtime"
	"slices"
	"strconv"
	"sync"

	"github.com/sanity-io/litter"

	"golang.org/x/sync/errgroup"

	enginetypes "github.com/projecteru2/core/engine/types"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resource/plugins/cpumem/schedule"
	cpumemtypes "github.com/projecteru2/core/resource/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const (
	fieldCapacity = "capacity"
	fieldUsage    = "usage"
)

type nodeResourceInfos struct {
	req       *cpumemtypes.NodeResourceRequest
	resource  *cpumemtypes.NodeResource
	workloads []*cpumemtypes.WorkloadResource
	info      *cpumemtypes.NodeResourceInfo
}

func (p Plugin) AddNode(ctx context.Context, nodename string, resource plugintypes.NodeResourceRequest, info *enginetypes.Info) (*plugintypes.AddNodeResponse, error) {
	var err error
	if _, err = p.doGetNodeResourceInfo(ctx, nodename); err == nil {
		return nil, coretypes.ErrNodeExists
	}

	if !p.store.NotFound(err) {
		log.WithFunc("resource.cpumem.AddNode").WithField("node", nodename).Error(ctx, err, "failed to get resource info of node")
		return nil, err
	}

	req := &cpumemtypes.NodeResourceRequest{}
	if err = req.Parse(p.config, resource); err != nil {
		return nil, err
	}

	if info != nil {
		if len(req.CPUMap) == 0 {
			req.CPUMap = cpumemtypes.CPUMap{}
			for i := range info.NCPU {
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

	if len(req.NUMA) > 0 && len(req.NUMAMemory) == 0 {
		numaNodes := slices.Compact(slices.Sorted(maps.Values(req.NUMA)))
		averageMemory := req.Memory / int64(len(numaNodes))
		nodeResourceInfo.Capacity.NUMAMemory = cpumemtypes.NUMAMemory{}
		for _, ID := range numaNodes {
			nodeResourceInfo.Capacity.NUMAMemory[ID] = averageMemory
		}
	}

	if err = p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		return nil, err
	}

	resp := &plugintypes.AddNodeResponse{}
	return resp, resourcetypes.Decode(map[string]any{
		fieldCapacity: nodeResourceInfo.Capacity,
		fieldUsage:    nodeResourceInfo.Usage,
	}, resp)
}

func (p Plugin) RemoveNode(ctx context.Context, nodename string) (*plugintypes.RemoveNodeResponse, error) {
	err := p.store.Delete(ctx, []string{fmt.Sprintf(nodeResourceInfoKey, nodename)})
	if err != nil {
		log.WithFunc("resource.cpumem.RemoveNode").WithField("node", nodename).Error(ctx, err, "failed to delete node")
	}
	return &plugintypes.RemoveNodeResponse{}, err
}

func (p Plugin) GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource plugintypes.WorkloadResourceRequest) (*plugintypes.GetNodesDeployCapacityResponse, error) {
	logger := log.WithFunc("resource.cpumem.GetNodesDeployCapacity")
	req := &cpumemtypes.WorkloadResourceRequest{}
	if err := req.Parse(resource); err != nil {
		return nil, err
	}
	if err := req.Validate(); err != nil {
		logger.Errorf(ctx, err, "invalid resource opts %+v", req)
		return nil, err
	}

	nodesResourceInfos, err := p.doGetNodesResourceInfo(ctx, nodenames)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	nodesDeployCapacityMap := make(map[string]*plugintypes.NodeDeployCapacity, len(nodenames))
	var planners errgroup.Group
	planners.SetLimit(runtime.GOMAXPROCS(0))
	for nodename, nodeResourceInfo := range nodesResourceInfos {
		planners.Go(func() error {
			if nodeDeployCapacity := p.doGetNodeDeployCapacity(nodeResourceInfo, req); nodeDeployCapacity.Capacity > 0 {
				mu.Lock()
				nodesDeployCapacityMap[nodename] = nodeDeployCapacity
				mu.Unlock()
			}
			return nil
		})
	}
	_ = planners.Wait()

	total := 0
	for _, nodeDeployCapacity := range nodesDeployCapacityMap {
		if total == math.MaxInt || nodeDeployCapacity.Capacity == math.MaxInt {
			total = math.MaxInt
		} else {
			total += nodeDeployCapacity.Capacity
		}
	}

	return &plugintypes.GetNodesDeployCapacityResponse{
		NodeDeployCapacityMap: nodesDeployCapacityMap,
		Total:                 total,
	}, nil
}

func (p Plugin) SetNodeResourceCapacity(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, delta, incr bool) (*plugintypes.SetNodeResourceCapacityResponse, error) {
	logger := log.WithFunc("resource.cpumem.SetNodeResourceCapacity").WithField("node", nodename)
	parsed, err := p.parseNodeResourceInfos(ctx, nodename, resource, resourceRequest, nil)
	if err != nil {
		return nil, err
	}
	req, nodeResource, nodeResourceInfo := parsed.req, parsed.resource, parsed.info
	origin := nodeResourceInfo.Capacity
	before := origin.DeepCopy()

	if !delta && req != nil {
		req.LoadFromOrigin(origin, resourceRequest)
	}
	nodeResourceInfo.Capacity = p.calculateNodeResource(req, nodeResource, origin, nil, delta, incr)

	for cpu := range nodeResourceInfo.Capacity.CPUMap {
		if _, ok := nodeResourceInfo.Usage.CPUMap[cpu]; !ok {
			nodeResourceInfo.Usage.CPUMap[cpu] = 0
		}
	}
	nodeResourceInfo.RemoveEmptyCores()

	if err := p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		logger.Errorf(ctx, err, "node resource info %+v", litter.Sdump(nodeResourceInfo))
		return nil, err
	}

	resp := &plugintypes.SetNodeResourceCapacityResponse{}
	return resp, resourcetypes.Decode(map[string]any{
		"before": before,
		"after":  nodeResourceInfo.Capacity,
	}, resp)
}

func (p Plugin) GetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error) {
	nodeResourceInfo, _, diffs, err := p.getNodeResourceInfo(ctx, nodename, workloadsResource)
	if err != nil {
		return nil, err
	}

	resp := &plugintypes.GetNodeResourceInfoResponse{}
	return resp, resourcetypes.Decode(map[string]any{
		fieldCapacity: nodeResourceInfo.Capacity,
		fieldUsage:    nodeResourceInfo.Usage,
		"diffs":       diffs,
	}, resp)
}

func (p Plugin) SetNodeResourceInfo(ctx context.Context, nodename string, capacity, usage plugintypes.NodeResource) (*plugintypes.SetNodeResourceInfoResponse, error) {
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

func (p Plugin) SetNodeResourceUsage(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, workloadsResource []plugintypes.WorkloadResource, delta, incr bool) (*plugintypes.SetNodeResourceUsageResponse, error) {
	logger := log.WithFunc("resource.cpumem.SetNodeResourceUsage").WithField("node", nodename)
	parsed, err := p.parseNodeResourceInfos(ctx, nodename, resource, resourceRequest, workloadsResource)
	if err != nil {
		return nil, err
	}
	req, nodeResource, wrksResource, nodeResourceInfo := parsed.req, parsed.resource, parsed.workloads, parsed.info
	origin := nodeResourceInfo.Usage
	before := origin.DeepCopy()

	nodeResourceInfo.Usage = p.calculateNodeResource(req, nodeResource, origin, wrksResource, delta, incr)

	if err := p.doSetNodeResourceInfo(ctx, nodename, nodeResourceInfo); err != nil {
		logger.Errorf(ctx, err, "node resource info %+v", litter.Sdump(nodeResourceInfo))
		return nil, err
	}

	resp := &plugintypes.SetNodeResourceUsageResponse{}
	return resp, resourcetypes.Decode(map[string]any{
		"before": before,
		"after":  nodeResourceInfo.Usage,
	}, resp)
}

func (p Plugin) GetMostIdleNode(ctx context.Context, nodenames []string) (*plugintypes.GetMostIdleNodeResponse, error) {
	var mostIdleNode string
	minIdle := math.MaxFloat64

	nodesResourceInfo, err := p.doGetNodesResourceInfo(ctx, nodenames)
	if err != nil {
		return nil, err
	}

	for nodename, nodeResourceInfo := range nodesResourceInfo {
		idle := float64(nodeResourceInfo.Usage.CPUMap.TotalPieces()) / float64(nodeResourceInfo.Capacity.CPUMap.TotalPieces())
		idle += float64(nodeResourceInfo.Usage.Memory) / float64(nodeResourceInfo.Capacity.Memory)

		if idle < minIdle {
			mostIdleNode = nodename
			minIdle = idle
		}
	}

	return &plugintypes.GetMostIdleNodeResponse{Nodename: mostIdleNode, Priority: priority}, nil
}

func (p Plugin) FixNodeResource(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error) {
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
	return resp, resourcetypes.Decode(map[string]any{
		fieldCapacity: nodeResourceInfo.Capacity,
		fieldUsage:    nodeResourceInfo.Usage,
		"diffs":       diffs,
	}, resp)
}

func (p Plugin) getNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*cpumemtypes.NodeResourceInfo, *cpumemtypes.WorkloadResource, []string, error) {
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
	resp, err := p.doGetNodesResourceInfo(ctx, []string{nodename})
	if err != nil {
		return nil, err
	}
	return resp[nodename], nil
}

func (p Plugin) doGetNodesResourceInfo(ctx context.Context, nodenames []string) (map[string]*cpumemtypes.NodeResourceInfo, error) {
	keys := make([]string, 0, len(nodenames))
	for _, nodename := range nodenames {
		keys = append(keys, fmt.Sprintf(nodeResourceInfoKey, nodename))
	}
	data, err := p.store.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	result := make(map[string]*cpumemtypes.NodeResourceInfo, len(data))
	for key, value := range data {
		r := &cpumemtypes.NodeResourceInfo{}
		if err := json.Unmarshal([]byte(value), r); err != nil {
			return nil, err
		}
		result[utils.Tail(key)] = r
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

	return p.store.Put(ctx, map[string]string{fmt.Sprintf(nodeResourceInfoKey, nodename): string(data)})
}

func (p Plugin) doGetNodeDeployCapacity(nodeResourceInfo *cpumemtypes.NodeResourceInfo, req *cpumemtypes.WorkloadResourceRequest) *plugintypes.NodeDeployCapacity {
	capacityInfo := &plugintypes.NodeDeployCapacity{
		Weight: 1,
	}
	if !req.CPUBind {
		if req.CPURequest > float64(len(nodeResourceInfo.Capacity.CPUMap)) {
			return capacityInfo
		}

		if req.MemRequest == 0 {
			capacityInfo.Capacity = math.MaxInt
			capacityInfo.Rate = 0
		} else {
			availableMemory := nodeResourceInfo.Capacity.Memory - nodeResourceInfo.Usage.Memory
			capacityInfo.Capacity = int(availableMemory / req.MemRequest)
			capacityInfo.Rate = utils.AdvancedDivide(float64(req.MemRequest), float64(nodeResourceInfo.Capacity.Memory))
		}
		capacityInfo.Usage = utils.AdvancedDivide(float64(nodeResourceInfo.Usage.Memory), float64(nodeResourceInfo.Capacity.Memory))
		return capacityInfo
	}

	capacityInfo.Capacity = schedule.CountCPUPlans(nodeResourceInfo, nil, p.config.Scheduler.ShareBase, p.config.Scheduler.MaxShare, req)
	capacityInfo.Usage = utils.AdvancedDivide(nodeResourceInfo.Usage.CPU, nodeResourceInfo.Capacity.CPU)
	capacityInfo.Rate = utils.AdvancedDivide(req.CPURequest, nodeResourceInfo.Capacity.CPU)
	capacityInfo.Weight = 100 // cpu-bind above all
	return capacityInfo
}

func (p Plugin) calculateNodeResource(req *cpumemtypes.NodeResourceRequest, nodeResource, origin *cpumemtypes.NodeResource, workloadsResource []*cpumemtypes.WorkloadResource, delta, incr bool) *cpumemtypes.NodeResource {
	var resp *cpumemtypes.NodeResource
	if origin == nil || !delta { // no delta means node resource rewrite with whole new data
		resp = &cpumemtypes.NodeResource{CPUMap: cpumemtypes.CPUMap{}, NUMAMemory: cpumemtypes.NUMAMemory{}, NUMA: cpumemtypes.NUMA{}}
		// a full rewrite must add onto the zero value; subtracting would store negative amounts
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

func (p Plugin) parseNodeResourceInfos(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, workloadsResource []plugintypes.WorkloadResource) (*nodeResourceInfos, error) {
	var req *cpumemtypes.NodeResourceRequest
	var nodeResource *cpumemtypes.NodeResource
	wrksResource := []*cpumemtypes.WorkloadResource{}

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

	for _, workloadResource := range workloadsResource {
		wrkResource := &cpumemtypes.WorkloadResource{}
		if err := wrkResource.Parse(workloadResource); err != nil {
			return nil, err
		}
		wrksResource = append(wrksResource, wrkResource)
	}

	nodeResourceInfo, err := p.doGetNodeResourceInfo(ctx, nodename)
	if err != nil {
		return nil, err
	}
	return &nodeResourceInfos{req: req, resource: nodeResource, workloads: wrksResource, info: nodeResourceInfo}, nil
}
