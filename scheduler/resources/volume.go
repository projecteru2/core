package resources

import "github.com/projecteru2/core/types"

type VolumeResourceRequest types.VolumeResourceRequest

func (r VolumeResourceRequest) MakeScheduler() types.SchedulerV2 {
	schedulerV1, err := complexscheduler.GetSchedulerV1()
	if err != nil {
		return
	}

	nodesInfo, total, err = schedulerV1.SelectVolumeNodes(nodesInfo, r.Volumes)
	return VolumeResourcePlans{volumePlans

	}, total, err
}

type VolumeResourcePlans struct {
	plans map[string][]types.VolumePlan
}

func (p VolumeResourcePlans) Type() types.ResourceType {
	return types.ResourceVolume
}

func (p VolumeResourcePlans) ApplyChangesOnNode(nodeInfo types.NodeInfo, node *types.Node) (err error) {
	volumeCost := types.VolumeMap{}
	for _, plan := range p.plans[nodeInfo.Name][:nodeInfo.Deploy] {
		volumeCost.Add(plan.IntoVolumeMap())
	}

	node.Volume.Sub(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.IncrUsage)
	return
}
