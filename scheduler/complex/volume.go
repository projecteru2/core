package complexscheduler

import (
	"math"

	"github.com/projecteru2/core/types"
)

func calculateVolumePlan(volumeMap types.VolumeMap, required []int64) (int, [][]types.VolumeMap) {
	if len(required) == 0 {
		return math.MaxInt32, nil
	}

	share := int(math.MaxInt64) // all fragments
	host := newHost(volumeMap, share)
	plans := host.distributeMultipleRations(required)
	cap := len(plans)
	if cap <= 0 {
		plans = nil
	}
	return cap, plans
}
