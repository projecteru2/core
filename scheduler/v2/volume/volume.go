package resources

import (
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

// VolumeResourceApply .
type VolumeResourceApply struct {
	request [32]types.VolumeBinding
	limit   [32]types.VolumeBinding
	length  int
}

// NewVolumeResourceRequest .
func NewVolumeApplication(request, limit types.VolumeBindings) (res VolumeResourceApply, _ error) {
	sort.Slice(request, func(i, j int) bool { return request[i].ToString(false) < request[j].ToString(false) })
	for i, vb := range request {
		res.request[i] = *vb
	}
	res.length = len(request)

	sort.Slice(limit, func(i, j int) bool { return limit[i].ToString(false) < limit[j].ToString(false) })
	for i, vb := range limit {
		res.limit[i] = *vb
	}
	return res, res.Validate()
}

// Type .
func (a VolumeResourceApply) Type() types.ResourceType {
	t := types.ResourceVolume
	for i := 0; i < a.length; i++ {
		if a.request[i].RequireSchedule() {
			t |= types.ResourceScheduledVolume
			break
		}
	}
	return t
}

// Validate .
func (a VolumeResourceApply) Validate() error {
	if len(a.request) == 0 && len(a.limit) > 0 {
		a.request = a.limit
	}
	if len(a.request) != len(a.limit) {
		return errors.Wrap(types.ErrBadVolume, "different length of request and limit")
	}
	for i := range a.request {
		req, lim := a.request[i], a.limit[i]
		if req.Source != lim.Source || req.Destination != lim.Destination || req.Flags != lim.Flags {
			return errors.Wrap(types.ErrBadVolume, "request and limit not match")
		}
		if req.SizeInBytes > 0 && lim.SizeInBytes > 0 && req.SizeInBytes > lim.SizeInBytes {
			return errors.Wrap(types.ErrBadVolume, "request size less than limit size ")
		}
	}
	return nil
}

// MakeScheduler .
func (a VolumeResourceApply) MakeScheduler() types.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans types.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		req, lim := types.VolumeBindings{}, types.VolumeBindings{}
		for i := 0; i < a.length; i++ {
			req = append(req, &a.request[i])
			lim = append(lim, &a.limit[i])
		}

		nodesInfo, volumePlans, total, err := schedulerV1.SelectVolumeNodes(nodesInfo, req)
		return VolumeResourcePlans{
			capacity: getCapacity(nodesInfo),
			req:      req,
			limit:    lim,
			Plans:    volumePlans,
		}, total, err
	}
}

// Rate .
func (a VolumeResourceApply) Rate(node types.Node) float64 {
	return float64(node.VolumeUsed) / float64(node.Volume.Total())
}

// VolumeResourcePlans .
type VolumeResourcePlans struct {
	capacity map[string]int
	req      types.VolumeBindings
	limit    types.VolumeBindings
	Plans    map[string][]types.VolumePlan
}

// Type .
func (p VolumeResourcePlans) Type() types.ResourceType {
	return types.ResourceVolume
}

// Capacity .
func (p VolumeResourcePlans) Capacity() map[string]int {
	return p.capacity
}

// ApplyChangesOnNode .
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

// RollbackChangesOnNode .
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

// Dispense .
func (p VolumeResourcePlans) Dispense(opts types.DispenseOptions, resources *types.Resources) error {
	if len(p.Plans) == 0 {
		return nil
	}

	resources.Volume = p.limit
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
		resources.Volume = append(resources.Volume, opts.HardVolumeBindings...)
	}

	// judge if volume changed
	if len(opts.ExistingInstances) > 0 && !resources.Volume.IsEqual(opts.ExistingInstances[opts.Index].Volumes) {
		resources.VolumeChanged = true
	}

	// fix plans while limit > request
	limitPlan := types.VolumePlan{}
	for i := range p.req {
		req, lim := p.req[i], p.limit[i]
		limitPlan[*lim] = resources.VolumePlan[*req]
		if lim.SizeInBytes > req.SizeInBytes {
			plan := resources.VolumePlan[*req]
			limitPlan[*lim] = types.VolumeMap{plan.GetResourceID(): plan.GetRation() + lim.SizeInBytes - req.SizeInBytes}
		}
	}
	resources.VolumePlan = limitPlan
	return nil
}
