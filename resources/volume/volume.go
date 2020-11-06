package volume

import (
	"sort"

	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// volumeResourceApply .
type volumeResourceApply struct {
	request [32]types.VolumeBinding
	limit   [32]types.VolumeBinding
	lenReq  int
	lenLim  int
}

// NewResourceRequirement .
func NewResourceRequirement(opts types.RawResourceOptions) (resourcetypes.ResourceRequirement, error) {
	a := &volumeResourceApply{}
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
}

// Type .
func (a volumeResourceApply) Type() types.ResourceType {
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
func (a *volumeResourceApply) Validate() error {
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
func (a volumeResourceApply) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
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
		return ResourcePlans{
			capacity: utils.GetCapacity(nodesInfo),
			req:      req,
			lim:      lim,
			PlanReq:  volumePlans,
		}, total, err
	}
}

// Rate .
func (a volumeResourceApply) Rate(node types.Node) float64 {
	return float64(node.VolumeUsed) / float64(node.Volume.Total())
}

// ResourcePlans .
type ResourcePlans struct {
	capacity map[string]int
	req      types.VolumeBindings
	lim      types.VolumeBindings
	PlanReq  map[string][]types.VolumePlan
}

// Type .
func (p ResourcePlans) Type() types.ResourceType {
	return types.ResourceVolume
}

// Capacity .
func (p ResourcePlans) Capacity() map[string]int {
	return p.capacity
}

// ApplyChangesOnNode .
func (p ResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	if len(p.PlanReq) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		plans, ok := p.PlanReq[node.Name]
		if !ok {
			continue
		}
		volumeCost.Add(plans[idx].IntoVolumeMap())
	}
	node.Volume.Sub(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.IncrUsage)
}

// RollbackChangesOnNode .
func (p ResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if len(p.PlanReq) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		volumeCost.Add(p.PlanReq[node.Name][idx].IntoVolumeMap())
	}
	node.Volume.Add(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.DecrUsage)
}

// Dispense .
func (p ResourcePlans) Dispense(opts resourcetypes.DispenseOptions, rsc *types.Resources) error {
	if len(p.PlanReq) == 0 {
		return nil
	}

	rsc.VolumeRequest = p.req
	rsc.VolumePlanRequest = p.PlanReq[opts.Node.Name][opts.Index]

	// if there are existing ones, ensure new volumes are compatible
	if len(opts.ExistingInstances) > 0 {
		plans := map[*types.Container]types.VolumePlan{}
	Searching:
		for _, plan := range p.PlanReq[opts.Node.Name] {
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

		rsc.VolumePlanRequest = plans[opts.ExistingInstances[opts.Index]]
	}

	// fix plans while limit > request
	rsc.VolumeLimit = p.lim
	rsc.VolumePlanLimit = types.VolumePlan{}
	for i := range p.req {
		req, lim := p.req[i], p.lim[i]
		if !req.RequireSchedule() {
			continue
		}
		if lim.SizeInBytes > req.SizeInBytes {
			p := rsc.VolumePlanRequest[*req]
			rsc.VolumePlanLimit[*lim] = types.VolumeMap{p.GetResourceID(): p.GetRation() + lim.SizeInBytes - req.SizeInBytes}
		} else {
			rsc.VolumePlanLimit[*lim] = rsc.VolumePlanRequest[*req]
		}
	}

	// append hard vbs
	if opts.HardVolumeBindings != nil {
		rsc.VolumeRequest = append(rsc.VolumeRequest, opts.HardVolumeBindings...)
		rsc.VolumeLimit = append(rsc.VolumeLimit, opts.HardVolumeBindings...)
	}

	// judge if volume changed
	if len(opts.ExistingInstances) > 0 && !rsc.VolumeLimit.IsEqual(opts.ExistingInstances[opts.Index].VolumeLimit) {
		rsc.VolumeChanged = true
	}
	return nil
}
