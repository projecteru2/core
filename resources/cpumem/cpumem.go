package cpumem

import (
	"strconv"

	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

func NewResourceRequirement(opts types.RawResourceOptions) (resourcetypes.ResourceRequirement, error) {
	a := &cpuMemRequirement{
		CPURequest:      opts.CPURequest,
		CPULimit:        opts.CPULimit,
		CPUBind:         opts.CPUBind,
		memoryRequest:   opts.MemoryRequest,
		memoryLimit:     opts.MemoryLimit,
		memorySoftLimit: opts.MemorySoft,
	}
	return a, a.Validate()
}

// cpuMemRequirement .
type cpuMemRequirement struct {
	CPURequest float64
	CPULimit   float64
	CPUBind    bool

	memoryRequest   int64
	memoryLimit     int64
	memorySoftLimit bool
}

// Type .
func (a cpuMemRequirement) Type() types.ResourceType {
	t := types.ResourceCPU | types.ResourceMemory
	if a.CPUBind {
		t |= types.ResourceCPUBind
	}
	return t
}

// Validate .
func (a *cpuMemRequirement) Validate() error {
	if a.memoryLimit < 0 || a.memoryRequest < 0 {
		return errors.Wrap(types.ErrBadMemory, "limit or request less than 0")
	}
	if a.memoryRequest == 0 && a.memoryLimit > 0 {
		a.memoryRequest = a.memoryLimit
	}
	if a.memoryLimit > 0 && a.memoryRequest > 0 && a.memoryRequest > a.memoryLimit {
		return errors.Wrap(types.ErrBadMemory, "limit less than request")
	}

	if a.CPULimit < 0 || a.CPURequest < 0 {
		return errors.Wrap(types.ErrBadCPU, "limit or request less than 0")
	}
	if a.CPURequest == 0 && a.CPULimit > 0 {
		a.CPURequest = a.CPULimit
	}
	if a.CPURequest > 0 && a.CPULimit > 0 && a.CPURequest > a.CPULimit {
		return errors.Wrap(types.ErrBadCPU, "limit less than request")
	}
	if a.CPURequest == 0 && a.CPUBind {
		return errors.Wrap(types.ErrBadCPU, "unlimited request with bind")
	}
	return nil
}

// MakeScheduler .
func (a cpuMemRequirement) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		var CPUPlans map[string][]types.CPUMap
		if !a.CPUBind || a.CPURequest == 0 {
			nodesInfo, total, err = schedulerV1.SelectMemoryNodes(nodesInfo, a.CPURequest, a.memoryRequest)
		} else {
			nodesInfo, CPUPlans, total, err = schedulerV1.SelectCPUNodes(nodesInfo, a.CPURequest, a.memoryRequest)
		}

		return CPUMemResourcePlans{
			memoryRequest:   a.memoryRequest,
			memoryLimit:     a.memoryLimit,
			memorySoftLimit: a.memorySoftLimit,
			CPURequest:      a.CPURequest,
			CPULimit:        a.CPULimit,
			CPUPlans:        CPUPlans,
			CPUBind:         a.CPUBind,
			capacity:        resourcetypes.GetCapacity(nodesInfo),
		}, total, err
	}
}

// Rate .
func (a cpuMemRequirement) Rate(node types.Node) float64 {
	if a.CPUBind {
		return a.CPURequest / float64(len(node.InitCPU))
	}
	return float64(a.memoryRequest) / float64(node.InitMemCap)
}

// CPUMemResourcePlans .
type CPUMemResourcePlans struct {
	memoryRequest   int64
	memoryLimit     int64
	memorySoftLimit bool

	CPURequest float64
	CPULimit   float64
	CPUPlans   map[string][]types.CPUMap
	CPUBind    bool

	capacity map[string]int
}

// Type .
func (p CPUMemResourcePlans) Type() (resourceType types.ResourceType) {
	resourceType = types.ResourceCPU | types.ResourceMemory
	if p.CPUPlans != nil {
		resourceType |= types.ResourceCPUBind
	}
	return resourceType
}

// Capacity .
func (p CPUMemResourcePlans) Capacity() map[string]int {
	return p.capacity
}

// ApplyChangesOnNode .
func (p CPUMemResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	if p.CPUPlans != nil {
		for _, idx := range indices {
			node.CPU.Sub(p.CPUPlans[node.Name][idx])
		}
	}
	node.MemCap -= p.memoryRequest * int64(len(indices))
	node.SetCPUUsed(p.CPURequest*float64(len(indices)), types.IncrUsage)
}

// RollbackChangesOnNode .
func (p CPUMemResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if p.CPUPlans != nil {
		for _, idx := range indices {
			node.CPU.Add(p.CPUPlans[node.Name][idx])
		}
	}
	node.MemCap += p.memoryRequest * int64(len(indices))
	node.SetCPUUsed(p.CPURequest*float64(len(indices)), types.DecrUsage)
}

// Dispense .
func (p CPUMemResourcePlans) Dispense(opts resourcetypes.DispenseOptions, rsc *types.Resources) error {
	rsc.CPUQuotaLimit = p.CPULimit
	rsc.CPUQuotaRequest = p.CPURequest
	rsc.CPUBind = p.CPUBind

	rsc.MemoryLimit = p.memoryLimit
	rsc.MemoryRequest = p.memoryRequest
	rsc.MemorySoftLimit = p.memorySoftLimit

	if len(p.CPUPlans) > 0 && p.CPULimit > 0 {
		if _, ok := p.CPUPlans[opts.Node.Name]; !ok {
			return errors.WithStack(types.ErrInsufficientCPU)
		}
		if len(p.CPUPlans[opts.Node.Name]) <= opts.Index {
			return errors.WithStack(types.ErrInsufficientCPU)
		}
		rsc.CPURequest = p.CPUPlans[opts.Node.Name][opts.Index]
		rsc.CPULimit = rsc.CPURequest
		rsc.NUMANode = opts.Node.GetNUMANode(rsc.CPURequest)
		return nil
	}

	// special handle when converting from cpu-binding to cpu-unbinding
	if len(opts.ExistingInstances) > opts.Index && len(opts.ExistingInstances[opts.Index].CPURequest) > 0 && len(p.CPUPlans) == 0 {
		rsc.CPURequest = types.CPUMap{}
		for i := 0; i < len(opts.Node.InitCPU); i++ {
			rsc.CPULimit[strconv.Itoa(i)] = 0
		}
	}
	return nil
}
