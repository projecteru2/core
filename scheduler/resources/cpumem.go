package resources

import (
	"strconv"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

// CPUMemResourceRequest .
type CPUMemResourceRequest struct {
	CPUQuota        float64
	CPUBind         bool
	Memory          int64
	MemorySoftLimit bool
}

// Type .
func (r CPUMemResourceRequest) Type() types.ResourceType {
	t := types.ResourceCPU | types.ResourceMemory
	if r.CPUBind {
		t |= types.ResourceCPUBind
	}
	return t
}

// DeployValidate .
func (r CPUMemResourceRequest) DeployValidate() error {
	if r.Memory < 0 {
		return types.NewDetailedErr(types.ErrBadMemory, r.Memory)
	}
	if r.CPUQuota < 0 {
		return types.NewDetailedErr(types.ErrBadCPU, r.CPUQuota)
	}
	return nil
}

// MakeScheduler .
func (r CPUMemResourceRequest) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans types.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		var CPUPlans map[string][]types.CPUMap
		if !r.CPUBind || r.CPUQuota == 0 {
			nodesInfo, total, err = schedulerV1.SelectMemoryNodes(nodesInfo, r.CPUQuota, r.Memory)
		} else {
			nodesInfo, CPUPlans, total, err = schedulerV1.SelectCPUNodes(nodesInfo, r.CPUQuota, r.Memory)
		}

		return CPUMemResourcePlans{
			memory:          r.Memory,
			memorySoftLimit: r.MemorySoftLimit,
			CPUQuota:        r.CPUQuota,
			CPUPlans:        CPUPlans,
			CPUBind:         r.CPUBind,
			capacity:        getCapacity(nodesInfo),
		}, total, err
	}
}

// Rate .
func (r CPUMemResourceRequest) Rate(node types.Node) float64 {
	if r.CPUBind {
		return r.CPUQuota / float64(len(node.InitCPU))
	}
	return float64(r.Memory) / float64(node.InitMemCap)
}

// CPUMemResourcePlans .
type CPUMemResourcePlans struct {
	memory          int64
	memorySoftLimit bool
	CPUQuota        float64
	CPUPlans        map[string][]types.CPUMap
	CPUBind         bool
	capacity        map[string]int
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
	node.MemCap -= p.memory * int64(len(indices))
	node.SetCPUUsed(p.CPUQuota*float64(len(indices)), types.IncrUsage)
}

// RollbackChangesOnNode .
func (p CPUMemResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if p.CPUPlans != nil {
		for _, idx := range indices {
			node.CPU.Add(p.CPUPlans[node.Name][idx])
		}
	}
	node.MemCap += p.memory * int64(len(indices))
	node.SetCPUUsed(p.CPUQuota*float64(len(indices)), types.DecrUsage)
}

// Dispense .
func (p CPUMemResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	resources.Quota = p.CPUQuota
	resources.Memory = p.memory
	resources.SoftLimit = p.memorySoftLimit
	resources.CPUBind = p.CPUBind

	if len(p.CPUPlans) > 0 {
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

	// special handle when convert from cpu-binding to cpu-unbinding
	if len(opts.ExistingInstances) > opts.Index && len(opts.ExistingInstances[opts.Index].CPU) > 0 && len(p.CPUPlans) == 0 {
		resources.CPU = types.CPUMap{}
		for i := 0; i < len(opts.Node.InitCPU); i++ {
			resources.CPU[strconv.Itoa(i)] = 0
		}
	}
	return nil
}
