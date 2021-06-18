package complexscheduler

import (
	"math"

	"github.com/projecteru2/core/types"
)

func calculateVolumePlan(volumeMap types.VolumeMap, required []int64) (int, [][]types.VolumeMap) {
	if len(required) == 0 {
		return math.MaxInt64, nil
	}

	share := int(math.MaxInt64) // all fragments
	host := newHost(volumeMap, share)
	plans := host.distributeMultipleRations(required)
	return len(plans), plans
}

func calculateMonopolyVolumePlan(initVolumeMap types.VolumeMap, volumeMap types.VolumeMap, required []int64) (cap int, plans [][]types.VolumeMap) {
	cap, rawPlans := calculateVolumePlan(volumeMap, required)
	if rawPlans == nil {
		return cap, nil
	}

	scheduled := map[string]bool{}
	for _, plan := range rawPlans {
		if !onSameSource(plan) {
			continue
		}

		volume := plan[0].GetResourceID()
		if _, ok := scheduled[volume]; ok {
			continue
		}

		plans = append(plans, proportionPlan(plan, initVolumeMap[volume]))
		scheduled[volume] = true
	}
	return len(plans), plans
}

func proportionPlan(plan []types.VolumeMap, size int64) (newPlan []types.VolumeMap) {
	var total int64
	for _, p := range plan {
		total += p.GetRation()
	}
	for _, p := range plan {
		newRation := int64(math.Floor(float64(p.GetRation()) / float64(total) * float64(size)))
		newPlan = append(newPlan, types.VolumeMap{p.GetResourceID(): newRation})
	}
	return
}

func distinguishVolumeBindings(vbs types.VolumeBindings) (norm, mono, unlim types.VolumeBindings) {
	for _, vb := range vbs {
		switch {
		case vb.RequireScheduleMonopoly():
			mono = append(mono, vb)
		case vb.RequireScheduleUnlimitedQuota():
			unlim = append(unlim, vb)
		case vb.RequireSchedule():
			norm = append(norm, vb)
		}
	}
	return
}

func distinguishAffinityVolumeBindings(vbs types.VolumeBindings, existing types.VolumePlan) (requireAff, remain types.VolumeBindings) {
	for _, vb := range vbs {
		_, _, found := existing.FindAffinityPlan(*vb)
		if found {
			requireAff = append(requireAff, vb)
		} else {
			remain = append(remain, vb)
		}
	}
	return
}
