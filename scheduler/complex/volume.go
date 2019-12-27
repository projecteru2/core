package complexscheduler

import (
	"math"

	"github.com/projecteru2/core/types"
)

func calculateVolumePlan(volumeMap types.VolumeMap, required []int) (int, [][]types.VolumeMap) {
	share := int(math.MaxInt64) // all fragments
	host := newHost(volumeMap, share)
	plans := host.distributeMultipleRations(required)
	cap := len(plans)
	if cap <= 0 {
		plans = nil
	}
	return cap, plans
}
