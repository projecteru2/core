package cpumem

import (
	"strconv"

	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type cpuMemRequest struct {
	CPUQuotaRequest float64
	CPUQuotaLimit   float64
	CPUBind         bool

	memoryRequest int64
	memoryLimit   int64
}

// MakeRequest .
func MakeRequest(opts types.ResourceOptions) (resourcetypes.ResourceRequest, error) {
	cmr := &cpuMemRequest{
		CPUQuotaRequest: opts.CPUQuotaRequest,
		CPUQuotaLimit:   opts.CPUQuotaLimit,
		CPUBind:         opts.CPUBind,
		memoryRequest:   opts.MemoryRequest,
		memoryLimit:     opts.MemoryLimit,
	}
	return cmr, cmr.Validate()
}

// Type .
func (cm cpuMemRequest) Type() types.ResourceType {
	t := types.ResourceCPU | types.ResourceMemory
	if cm.CPUBind {
		t |= types.ResourceCPUBind
	}
	return t
}

// Validate .
func (cm *cpuMemRequest) Validate() error {
	if cm.memoryLimit < 0 || cm.memoryRequest < 0 {
		return errors.Wrap(types.ErrBadMemory, "limit or request less than 0")
	}
	if cm.CPUQuotaLimit < 0 || cm.CPUQuotaRequest < 0 {
		return errors.Wrap(types.ErrBadCPU, "limit or request less than 0")
	}
	if cm.CPUQuotaRequest == 0 && cm.CPUBind {
		return errors.Wrap(types.ErrBadCPU, "unlimited request with bind")
	}

	if cm.memoryRequest == 0 && cm.memoryLimit > 0 {
		cm.memoryRequest = cm.memoryLimit
	}
	// 如果需求量大于限制量，悄咪咪的把限制量抬到需求量的水平，做成名义上的软限制
	if cm.memoryLimit > 0 && cm.memoryRequest > 0 && cm.memoryRequest > cm.memoryLimit {
		cm.memoryLimit = cm.memoryRequest
	}
	if cm.CPUQuotaRequest == 0 && cm.CPUQuotaLimit > 0 {
		cm.CPUQuotaRequest = cm.CPUQuotaLimit
	}
	if cm.CPUQuotaRequest > 0 && cm.CPUQuotaLimit > 0 && cm.CPUQuotaRequest > cm.CPUQuotaLimit {
		cm.CPUQuotaLimit = cm.CPUQuotaRequest
	}
	return nil
}

// MakeScheduler .
func (cm cpuMemRequest) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		var CPUPlans map[string][]types.CPUMap
		if !cm.CPUBind || cm.CPUQuotaRequest == 0 {
			nodesInfo, total, err = schedulerV1.SelectMemoryNodes(nodesInfo, cm.CPUQuotaRequest, cm.memoryRequest)
		} else {
			nodesInfo, CPUPlans, total, err = schedulerV1.SelectCPUNodes(nodesInfo, cm.CPUQuotaRequest, cm.memoryRequest)
		}

		return ResourcePlans{
			memoryRequest:   cm.memoryRequest,
			memoryLimit:     cm.memoryLimit,
			CPUQuotaRequest: cm.CPUQuotaRequest,
			CPUQuotaLimit:   cm.CPUQuotaLimit,
			CPUPlans:        CPUPlans,
			capacity:        utils.GetCapacity(nodesInfo),
		}, total, err
	}
}

// Rate for global strategy
func (cm cpuMemRequest) Rate(node types.Node) float64 {
	if cm.CPUBind {
		return cm.CPUQuotaRequest / float64(len(node.InitCPU))
	}
	return float64(cm.memoryRequest) / float64(node.InitMemCap)
}

// ResourcePlans .
type ResourcePlans struct {
	memoryRequest int64
	memoryLimit   int64

	CPUQuotaRequest float64
	CPUQuotaLimit   float64
	CPUPlans        map[string][]types.CPUMap

	capacity map[string]int
}

// Type .
func (rp ResourcePlans) Type() (resourceType types.ResourceType) {
	resourceType = types.ResourceCPU | types.ResourceMemory
	if rp.CPUPlans != nil {
		resourceType |= types.ResourceCPUBind
	}
	return resourceType
}

// Capacity .
func (rp ResourcePlans) Capacity() map[string]int {
	return rp.capacity
}

// ApplyChangesOnNode .
func (rp ResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	if rp.CPUPlans != nil {
		for _, idx := range indices {
			node.CPU.Sub(rp.CPUPlans[node.Name][idx])
		}
	}
	node.MemCap -= rp.memoryRequest * int64(len(indices))
	node.SetCPUUsed(rp.CPUQuotaRequest*float64(len(indices)), types.IncrUsage)
}

// RollbackChangesOnNode .
func (rp ResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if rp.CPUPlans != nil {
		for _, idx := range indices {
			node.CPU.Add(rp.CPUPlans[node.Name][idx])
		}
	}
	node.MemCap += rp.memoryRequest * int64(len(indices))
	node.SetCPUUsed(rp.CPUQuotaRequest*float64(len(indices)), types.DecrUsage)
}

// Dispense .
func (rp ResourcePlans) Dispense(opts resourcetypes.DispenseOptions, r *types.Resource1) (*types.Resource1, error) {
	r.CPUQuotaLimit = rp.CPUQuotaLimit
	r.CPUQuotaRequest = rp.CPUQuotaRequest
	r.MemoryLimit = rp.memoryLimit
	r.MemoryRequest = rp.memoryRequest

	if len(rp.CPUPlans) > 0 {
		if _, ok := rp.CPUPlans[opts.Node.Name]; !ok {
			return nil, errors.WithStack(types.ErrInsufficientCPU)
		}
		if len(rp.CPUPlans[opts.Node.Name]) <= opts.Index {
			return nil, errors.WithStack(types.ErrInsufficientCPU)
		}
		r.CPU = rp.CPUPlans[opts.Node.Name][opts.Index]
		r.NUMANode = opts.Node.GetNUMANode(r.CPU)
	}

	// special handle when converting from cpu-binding to cpu-unbinding
	if len(opts.ExistingInstances) > opts.Index && len(opts.ExistingInstances[opts.Index].CPU) > 0 && len(rp.CPUPlans) == 0 {
		r.CPU = types.CPUMap{}
		for i := 0; i < len(opts.Node.InitCPU); i++ {
			r.CPU[strconv.Itoa(i)] = 0
		}
	}
	return r, nil
}
