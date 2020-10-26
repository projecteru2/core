package resources

import (
	"sort"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/resources"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

// VolumeResourceApply .
type VolumeResourceApply struct {
	request [32]types.VolumeBinding
	limit   [32]types.VolumeBinding
	lenReq  int
	lenLim  int
}

// NewVolumeResourceRequest .
func init() {
	resources.RegisterApplicationFactory(func(opts types.RawResourceOptions) (resources.ResourceApplication, error) {
		a := &VolumeResourceApply{}
		sort.Slice(opts.VolumeRequest, func(i, j int) bool {
			return opts.VolumeRequest[i].ToString(false) < opts.VolumeRequest[j].ToString(false)
		})
		for i, vb := range opts.VolumeRequest {
			a.request[i] = *vb
		}
		a.lenReq = len(opts.VolumeRequest)
		a.lenLim = len(opts.VolumeLimit)

		sort.Slice(opts.VolumeLimit, func(i, j int) bool { return opts.VolumeLimit[i].ToString(false) < opts.VolumeLimit[j].ToString(false) })
		for i, vb := range opts.VolumeLimit {
			a.limit[i] = *vb
		}
		return a, a.Validate()
	})
}

// Type .
func (a VolumeResourceApply) Type() types.ResourceType {
	t := types.ResourceVolume
	for i := 0; i < a.lenReq; i++ {
		if a.request[i].RequireSchedule() {
			t |= types.ResourceScheduledVolume
			break
		}
	}
	return t
}

// Validate .
func (a *VolumeResourceApply) Validate() error {
	if a.lenReq == 0 && a.lenLim > 0 {
		a.request = a.limit
		a.lenReq = a.lenLim
	}
	if a.lenReq != a.lenLim {
		return errors.Wrap(types.ErrBadVolume, "different length of request and limit")
	}
	for i := 0; i < a.lenReq; i++ {
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
func (a VolumeResourceApply) MakeScheduler() resources.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans resources.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		req, lim := types.VolumeBindings{}, types.VolumeBindings{}
		for i := 0; i < a.lenReq; i++ {
			req = append(req, &a.request[i])
			lim = append(lim, &a.limit[i])
		}

		nodesInfo, volumePlans, total, err := schedulerV1.SelectVolumeNodes(nodesInfo, req)
		return VolumeResourcePlans{
			capacity: resources.GetCapacity(nodesInfo),
			req:      req,
			lim:      lim,
			planReq:  volumePlans,
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
	lim      types.VolumeBindings
	planReq  map[string][]types.VolumePlan
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
	if len(p.planReq) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		volumeCost.Add(p.planReq[node.Name][idx].IntoVolumeMap())
	}
	node.Volume.Sub(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.IncrUsage)
}

// RollbackChangesOnNode .
func (p VolumeResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if len(p.planReq) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		volumeCost.Add(p.planReq[node.Name][idx].IntoVolumeMap())
	}
	node.Volume.Add(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.DecrUsage)
}

// Dispense .
func (p VolumeResourcePlans) Dispense(opts resources.DispenseOptions, resources *types.Resources) error {
	if len(p.planReq) == 0 {
		return nil
	}

	resources.VolumeRequest = p.req
	resources.VolumePlanRequest = p.planReq[opts.Node.Name][opts.Index]

	// if there are existing ones, ensure new volumes are compatible
	if len(opts.ExistingInstances) > 0 {
		plans := map[*types.Container]types.VolumePlan{}
	Searching:
		for _, plan := range p.planReq[opts.Node.Name] {
			for _, container := range opts.ExistingInstances {
				if _, ok := plans[container]; !ok && plan.Compatible(container.VolumePlanRequest) {
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

		resources.VolumePlanRequest = plans[opts.ExistingInstances[opts.Index]]
	}

	// append hard vbs
	if opts.HardVolumeBindings != nil {
		resources.VolumeRequest = append(resources.VolumeRequest, opts.HardVolumeBindings...)
	}

	// fix plans while limit > request
	resources.VolumeLimit = resources.VolumeRequest
	resources.VolumePlanLimit = resources.VolumePlanRequest
	planLimit := types.VolumePlan{}
	// TODO@zc: hard req in p.req
	for i := range p.req {
		req, lim := p.req[i], p.lim[i]
		if lim.SizeInBytes > req.SizeInBytes {
			p := resources.VolumePlanRequest[*req]
			planLimit[*lim] = types.VolumeMap{p.GetResourceID(): p.GetRation() + lim.SizeInBytes - req.SizeInBytes}
		} else {
			planLimit[*lim] = resources.VolumePlanRequest[*req]
		}
	}

	// judge if volume changed
	if len(opts.ExistingInstances) > 0 && !resources.VolumeLimit.IsEqual(opts.ExistingInstances[opts.Index].VolumeLimit) {
		resources.VolumeChanged = true
	}
	return nil
}
