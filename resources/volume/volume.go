package volume

import (
	"sort"

	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const maxVolumes = 32

// volumeResourceRequirement .
type volumeResourceRequirement struct {
	request  [maxVolumes]types.VolumeBinding
	limit    [maxVolumes]types.VolumeBinding
	requests int
	limits   int
}

// NewResourceRequirement .
func NewResourceRequirement(opts types.Resource) (resourcetypes.ResourceRequirement, error) {
	v := &volumeResourceRequirement{}
	sort.Slice(opts.VolumeRequest, func(i, j int) bool {
		return opts.VolumeRequest[i].ToString(false) < opts.VolumeRequest[j].ToString(false)
	})
	for i, vb := range opts.VolumeRequest {
		v.request[i] = *vb
	}
	v.requests = len(opts.VolumeRequest)
	v.limits = len(opts.VolumeLimit)

	sort.Slice(opts.VolumeLimit, func(i, j int) bool {
		return opts.VolumeLimit[i].ToString(false) < opts.VolumeLimit[j].ToString(false)
	})
	for i, vb := range opts.VolumeLimit {
		v.limit[i] = *vb
	}
	return v, v.Validate()
}

// Type .
func (v volumeResourceRequirement) Type() types.ResourceType {
	t := types.ResourceVolume
	for i := 0; i < v.requests; i++ {
		if v.request[i].RequireSchedule() {
			t |= types.ResourceScheduledVolume
			break
		}
	}
	return t
}

// Validate .
func (v *volumeResourceRequirement) Validate() error {
	if v.requests == 0 && v.limits > 0 {
		v.request = v.limit
		v.requests = v.limits
	}
	if v.requests != v.limits {
		return errors.Wrap(types.ErrBadVolume, "different length of request and limit")
	}
	for i := 0; i < v.requests; i++ {
		req, lim := v.request[i], v.limit[i]
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
func (v volumeResourceRequirement) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(nodesInfo []types.NodeInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		request, limit := types.VolumeBindings{}, types.VolumeBindings{}
		for i := 0; i < v.requests; i++ {
			request = append(request, &v.request[i])
			limit = append(limit, &v.limit[i])
		}

		nodesInfo, volumePlans, total, err := schedulerV1.SelectVolumeNodes(nodesInfo, request)
		return ResourcePlans{
			capacity: utils.GetCapacity(nodesInfo),
			request:  request,
			limit:    limit,
			plan:     volumePlans,
		}, total, err
	}
}

// Rate .
func (v volumeResourceRequirement) Rate(node types.Node) float64 {
	return float64(node.VolumeUsed) / float64(node.Volume.Total())
}

// ResourcePlans .
type ResourcePlans struct {
	capacity map[string]int
	request  types.VolumeBindings
	limit    types.VolumeBindings
	plan     map[string][]types.VolumePlan
}

// Type .
func (rp ResourcePlans) Type() types.ResourceType {
	return types.ResourceVolume
}

// Capacity .
func (rp ResourcePlans) Capacity() map[string]int {
	return rp.capacity
}

// ApplyChangesOnNode .
func (rp ResourcePlans) ApplyChangesOnNode(node *types.Node, indices ...int) {
	if len(rp.plan) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		plans, ok := rp.plan[node.Name]
		if !ok {
			continue
		}
		volumeCost.Add(plans[idx].IntoVolumeMap())
	}
	node.Volume.Sub(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.IncrUsage)
}

// RollbackChangesOnNode .
func (rp ResourcePlans) RollbackChangesOnNode(node *types.Node, indices ...int) {
	if len(rp.plan) == 0 {
		return
	}

	volumeCost := types.VolumeMap{}
	for _, idx := range indices {
		volumeCost.Add(rp.plan[node.Name][idx].IntoVolumeMap())
	}
	node.Volume.Add(volumeCost)
	node.SetVolumeUsed(volumeCost.Total(), types.DecrUsage)
}

// Dispense .
func (rp ResourcePlans) Dispense(opts resourcetypes.DispenseOptions) (*types.Resource, error) {
	r := &types.Resource{}
	if len(rp.plan) == 0 {
		return r, nil
	}

	r.VolumeRequest = rp.req
	r.VolumePlanRequest = rp.PlanReq[opts.Node.Name][opts.Index]

	// if there are existing ones, ensure new volumes are compatible
	if len(opts.ExistingInstances) > 0 {
		plans := map[*types.Container]types.VolumePlan{}
	Searching:
		for _, plan := range rp.PlanReq[opts.Node.Name] {
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
	rsc.VolumeLimit = rp.lim
	rsc.VolumePlanLimit = types.VolumePlan{}
	for i := range rp.req {
		req, lim := rp.req[i], rp.lim[i]
		if !req.RequireSchedule() {
			continue
		}
		if lim.SizeInBytes > req.SizeInBytes {
			p := rsc.VolumePlanRequest[*req]
			rsc.VolumePlanLimit[*lim] = types.VolumeMap{rp.GetResourceID(): rp.GetRation() + lim.SizeInBytes - req.SizeInBytes}
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
