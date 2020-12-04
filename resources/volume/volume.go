package volume

import (
	"sort"

	"github.com/pkg/errors"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
)

const maxVolumes = 32

type volumeRequest struct {
	request  [maxVolumes]types.VolumeBinding
	limit    [maxVolumes]types.VolumeBinding
	requests int
	limits   int
}

// MakeRequest .
func MakeRequest(opts types.ResourceOptions) (resourcetypes.ResourceRequest, error) {
	v := &volumeRequest{}
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
func (v volumeRequest) Type() types.ResourceType {
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
func (v *volumeRequest) Validate() error {
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
			v.limit[i].SizeInBytes = req.SizeInBytes
		}
	}
	return nil
}

// MakeScheduler .
func (v volumeRequest) MakeScheduler() resourcetypes.SchedulerV2 {
	return func(scheduleInfos []resourcetypes.ScheduleInfo) (plans resourcetypes.ResourcePlans, total int, err error) {
		schedulerV1, err := scheduler.GetSchedulerV1()
		if err != nil {
			return
		}

		request, limit := types.VolumeBindings{}, types.VolumeBindings{}
		for i := 0; i < v.requests; i++ {
			request = append(request, &v.request[i])
			limit = append(limit, &v.limit[i])
		}

		scheduleInfos, volumePlans, total, err := schedulerV1.SelectVolumeNodes(scheduleInfos, request)
		return ResourcePlans{
			capacity: resourcetypes.GetCapacity(scheduleInfos),
			request:  request,
			limit:    limit,
			plan:     volumePlans,
		}, total, err
	}
}

// Rate .
func (v volumeRequest) Rate(node types.Node) float64 {
	var totalRequest int64
	for i := 0; i < v.requests; i++ {
		totalRequest += v.request[i].SizeInBytes
	}
	return float64(totalRequest) / float64(node.Volume.Total())
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
func (rp ResourcePlans) Dispense(opts resourcetypes.DispenseOptions, r *types.ResourceMeta) (*types.ResourceMeta, error) {
	if len(rp.plan) == 0 {
		return r, nil
	}

	r.VolumeRequest = rp.request
	r.VolumePlanRequest = rp.plan[opts.Node.Name][opts.Index]

	// if there are existing ones, ensure new volumes are compatible
	if opts.ExistingInstance != nil {
		found := false
		for _, plan := range rp.plan[opts.Node.Name] {
			if plan.Compatible(opts.ExistingInstance.VolumePlanRequest) {
				r.VolumePlanRequest = plan
				found = true
				break
			}
		}

		if !found {
			return nil, errors.Wrap(types.ErrInsufficientVolume, "incompatible volume plans")
		}
	}

	// fix plans while limit > request
	r.VolumeLimit = rp.limit
	r.VolumePlanLimit = types.VolumePlan{}
	for i := range rp.request {
		request, limit := rp.request[i], rp.limit[i]
		if !request.RequireSchedule() {
			continue
		}
		if limit.SizeInBytes > request.SizeInBytes {
			p := r.VolumePlanRequest[*request]
			r.VolumePlanLimit[*limit] = types.VolumeMap{p.GetResourceID(): p.GetRation() + limit.SizeInBytes - request.SizeInBytes}
		} else {
			r.VolumePlanLimit[*limit] = r.VolumePlanRequest[*request]
		}
	}

	// judge if volume changed
	r.VolumeChanged = opts.ExistingInstance != nil && !r.VolumeLimit.IsEqual(opts.ExistingInstance.VolumeLimit)
	return r, nil
}

// GetPlan return volume plans by nodename
func (rp ResourcePlans) GetPlan(nodename string) []types.VolumePlan {
	return rp.plan[nodename]
}
