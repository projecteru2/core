package resources

import (
	complexscheduler "github.com/projecteru2/core/scheduler/complex"
	"github.com/projecteru2/core/types"
)

type CPUMemResourceRequest types.CPUMemResourceRequest

func (r CPUMemResourceRequest) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans CPUMemResourcePlans, total int, err error) {
		schedulerV1, err := complexscheduler.GetSchedulerV1()
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
			memory:   r.Memory,
			CPUQuota: r.CPUQuota,
			CPUPlans: CPUPlans,
		}, total, err
	}
}

type CPUMemResourcePlans struct {
	memory   int64
	CPUQuota float64
	CPUPlans map[string][]types.CPUMap
}

func (p CPUMemResourcePlans) Type() types.ResourceType {
	return types.ResourceCPU | types.ResourceMemory
}

func (p CPUMemResourcePlans) ApplyChangesOnNode(nodeInfo types.NodeInfo, node *types.Node) (err error) {
	quotaCost := p.CPUQuota * float64(nodeInfo.Deploy)
	memoryCost := p.memory * int64(nodeInfo.Deploy)

	CPUCost := types.Map{}
	for _, CPU := range p.CPUPlans[nodeInfo.Name][:nodeInfo.Deploy] {
		CPUCost.Add(CPU)
	}

	node.CPU.Sub(CPUCost)
	node.MemCap -= memoryCost
	node.SetCPUUsed(quotaCost, types.IncrUsage)
	return
}
