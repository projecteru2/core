package resources

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/scheduler/v2/resources"
	"github.com/projecteru2/core/types"
)

func init() {
	resources.RegisterApplicationFactory(func(opts types.RawResourceOptions) (resources.ResourceApplication, error) {
		a := &CPUMemApplication{
			CPURequest:      opts.CPURequest,
			CPULimit:        opts.CPULimit,
			CPUBind:         opts.CPUBind,
			memoryRequest:   opts.MemoryRequest,
			memoryLimit:     opts.MemoryLimit,
			memorySoftLimit: opts.MemorySoft,
		}
		return a, a.Validate()
	})
}

// CPUMemApplication .
type CPUMemApplication struct {
	CPURequest float64
	CPULimit   float64
	CPUBind    bool

	memoryRequest   int64
	memoryLimit     int64
	memorySoftLimit bool
}

// Type .
func (a CPUMemApplication) Type() types.ResourceType {
	t := types.ResourceCPU | types.ResourceMemory
	if a.CPUBind {
		t |= types.ResourceCPUBind
	}
	return t
}

// Validate .
func (a CPUMemApplication) Validate() error {
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
func (a CPUMemApplication) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans types.ResourcePlans, total int, err error) {
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
			capacity:        getCapacity(nodesInfo),
		}, total, err
	}
}

// Rate .
func (a CPUMemApplication) Rate(node types.Node) float64 {
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
func (p CPUMemResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	resources.Quota = p.CPULimit
	resources.Memory = p.memoryLimit
	resources.SoftLimit = p.memorySoftLimit
	resources.CPUBind = p.CPUBind

	if len(p.CPUPlans) > 0 && p.CPULimit > 0 {
		if _, ok := p.CPUPlans[opts.Node.Name]; !ok {
			return errors.WithStack(types.ErrInsufficientCPU)
		}
		if len(p.CPUPlans[opts.Node.Name]) <= opts.Index {
			return errors.WithStack(types.ErrInsufficientCPU)
		}
		resources.CPU = p.CPUPlans[opts.Node.Name][opts.Index]
		resources.NUMANode = opts.Node.GetNUMANode(resources.CPU)
		return nil
	}

	// special handle when converting from cpu-binding to cpu-unbinding
	if len(opts.ExistingInstances) > opts.Index && len(opts.ExistingInstances[opts.Index].CPU) > 0 && len(p.CPUPlans) == 0 {
		resources.CPU = types.CPUMap{}
		for i := 0; i < len(opts.Node.InitCPU); i++ {
			resources.CPU[strconv.Itoa(i)] = 0
		}
	}
	return nil
}
