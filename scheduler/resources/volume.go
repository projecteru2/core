package resources

import (
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

type VolumeResourceRequest struct {
	vbs    [32]types.VolumeBinding
	length int
}

func NewVolumeResourceRequest(vbs types.VolumeBindings) (res VolumeResourceRequest) {
	sort.Slice(vbs, func(i, j int) bool { return vbs[i].ToString(false) < vbs[j].ToString(false) })
	for i, vb := range vbs {
		res.vbs[i] = *vb
	}
	res.length = len(vbs)
	return
}

func (r VolumeResourceRequest) Type() types.ResourceType {
	t := types.ResourceVolume
	for i := 0; i < r.length; i++ {
		if r.vbs[i].RequireSchedule() {
			t |= types.ResourceVolumeNeedSchedule
			break
		}
	}
	return t
}

func (r VolumeResourceRequest) DeployValidate() error { return nil }

func (r VolumeResourceRequest) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans types.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		vbs := types.VolumeBindings{}
		for i := 0; i < r.length; i++ {
			vbs = append(vbs, &r.vbs[i])
		}
		nodesInfo, volumePlans, total, err := schedulerV1.SelectVolumeNodes(nodesInfo, vbs)
		return VolumeResourcePlans{
			capacity: getCapacity(nodesInfo),
			req:      vbs,
			Plans:    volumePlans,
		}, total, err
	}
}

func (r VolumeResourceRequest) Rate(node types.Node) float64 {
	return float64(node.VolumeUsed) / float64(node.Volume.Total())
}

type VolumeResourcePlans struct {
	capacity map[string]int
	req      types.VolumeBindings
	Plans    map[string][]types.VolumePlan
}

func (p VolumeResourcePlans) Type() types.ResourceType {
	return types.ResourceVolume
}

func (p VolumeResourcePlans) Capacity() map[string]int {
	return p.capacity
}

func (p VolumeResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	if len(p.Plans) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		volumeCost.Add(p.Plans[node.Name][idx].IntoVolumeMap())
	}
	node.Volume.Sub(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.IncrUsage)
}

func (p VolumeResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if len(p.Plans) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		volumeCost.Add(p.Plans[node.Name][idx].IntoVolumeMap())
	}
	node.Volume.Add(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.DecrUsage)
}

func (p VolumeResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	if len(p.Plans) == 0 {
		return nil
	}

	resources.Volume = p.req
	resources.VolumePlan = p.Plans[opts.Node.Name][opts.Index]

	// if there are existing ones, ensure new volumes are compatible
	if len(opts.ExistingInstances) > 0 {
		plans := map[*types.Container]types.VolumePlan{}
	Searching:
		for _, plan := range p.Plans[opts.Node.Name] {
			for _, container := range opts.ExistingInstances {
				if _, ok := plans[container]; !ok && plan.Compatible(container.VolumePlan) {
					plans[container] = plan
					if len(plans) == len(opts.ExistingInstances) {
						break Searching
					}
					break
				}
			}
		}

		if len(plans) < len(opts.ExistingInstances) {
			return errors.Wrap(types.ErrInsufficientVolume, "incompatible volume plans")
		}

		resources.VolumePlan = plans[opts.ExistingInstances[opts.Index]]
	}

	// append hard vbs
	if opts.HardVolumeBindings != nil {
		resources.Volume = types.MustToVolumeBindings(append(resources.Volume.ToStringSlice(false, false), opts.HardVolumeBindings.ToStringSlice(false, false)...))
	}

	// judge if volume changed
	if len(opts.ExistingInstances) > 0 && resources.Volume.IsEqual(opts.ExistingInstances[opts.Index].Volumes) {
		resources.VolumeChanged = true
	}
	return nil
}
