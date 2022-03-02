package schedule

import (
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/volume/types"
)

var maxDeployCount = 1000

func generateResourceInfo() *types.NodeResourceInfo {
	return &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			Volumes: types.VolumeMap{
				"/data0": units.TiB,
				"/data1": units.TiB,
				"/data2": units.TiB,
				"/data3": units.TiB,
			},
		},
		Usage: &types.NodeResourceArgs{
			Volumes: types.VolumeMap{
				"/data0": 200 * units.GiB,
				"/data1": 300 * units.GiB,
			},
		},
	}
}

func generateEmptyResourceInfo() *types.NodeResourceInfo {
	return &types.NodeResourceInfo{
		Capacity: &types.NodeResourceArgs{
			Volumes: types.VolumeMap{},
		},
		Usage: &types.NodeResourceArgs{
			Volumes: types.VolumeMap{},
		},
	}
}

func applyPlans(resourceInfo *types.NodeResourceInfo, plans []types.VolumePlan) {
	for _, plan := range plans {
		for _, volumeMap := range plan {
			for device, size := range volumeMap {
				resourceInfo.Usage.Volumes[device] += size
			}
		}
	}
}

func noMorePlans(t *testing.T, resourceInfo *types.NodeResourceInfo, volumePlans []types.VolumePlan, volumeRequest types.VolumeBindings) {
	applyPlans(resourceInfo, volumePlans)
	assert.Nil(t, resourceInfo.Validate())
	plan := GetVolumePlans(resourceInfo, volumeRequest, maxDeployCount)
	assert.Equal(t, len(plan), 0)
}

func validateVolumePlan(t *testing.T, resourceInfo *types.NodeResourceInfo, volumeRequest types.VolumeBindings, volumePlan types.VolumePlan) {
	t.Logf("volume plan: %v", volumePlan)
	t.Logf("volume request: %v", volumeRequest)
	monoDevice := ""
	monoTotalSize := int64(0)
	for _, binding := range volumeRequest {
		if !binding.RequireSchedule() {
			continue
		}
		volumeMap, ok := volumePlan[binding]
		assert.True(t, ok)
		switch {
		case binding.RequireScheduleMonopoly():
			if monoDevice == "" {
				monoDevice = volumeMap.GetDevice()
			}
			assert.Equal(t, monoDevice, volumeMap.GetDevice())
			monoTotalSize += volumeMap.GetSize()
		case binding.RequireSchedule():
			assert.Equal(t, volumeMap.GetSize(), binding.SizeInBytes)
		}
	}

	assert.Equal(t, monoTotalSize, resourceInfo.Capacity.Volumes[monoDevice])
}

func validateVolumePlans(t *testing.T, resourceInfo *types.NodeResourceInfo, volumeRequest types.VolumeBindings, volumePlans []types.VolumePlan) {
	t.Logf("%v plans in total", len(volumePlans))
	t.Logf("plans: %v", volumePlans)
	for _, plan := range volumePlans {
		validateVolumePlan(t, resourceInfo, volumeRequest, plan)
	}
	noMorePlans(t, resourceInfo, volumePlans, volumeRequest)
}

func generateVolumeBindings(t *testing.T, str []string) types.VolumeBindings {
	bindings, err := types.NewVolumeBindings(str)
	assert.Nil(t, err)
	return bindings
}

func TestGetVolumePlans(t *testing.T) {
	// no volume
	resourceInfo := generateEmptyResourceInfo()

	// single normal request
	volumeRequest, err := types.NewVolumeBindings([]string{})
	assert.Nil(t, err)

	plans := GetVolumePlans(resourceInfo, volumeRequest, maxDeployCount)
	assert.Equal(t, len(plans), 0)

	// normal cases
	requests := []types.VolumeBindings{
		// single normal request
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:500GiB",
		}),
		// multiple normal request
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:500GiB",
			"AUTO:/dir2:rw:500GiB",
		}),
		// with single mono request
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:1GiB",
			"AUTO:/dir2:rwm:1GiB",
		}),
		// with mono requests
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:500GiB",
			"AUTO:/dir2:rw:500GiB",
			"AUTO:/dir3:rwm:100GiB",
			"AUTO:/dir4:rwm:100GiB",
			"AUTO:/dir5:rwm:100GiB",
		}),
		// with unlimited requests
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:500GiB",
			"AUTO:/dir2:rw:0",
			"AUTO:/dir3:rwm:100GiB",
			"AUTO:/dir4:rwm:100GiB",
			"AUTO:/dir5:rwm:100GiB",
		}),
	}

	for _, volumeRequest := range requests {
		resourceInfo = generateResourceInfo()
		plans = GetVolumePlans(resourceInfo, volumeRequest, maxDeployCount)
		validateVolumePlans(t, resourceInfo, volumeRequest, plans)
	}

	// invalid requests
	requests = []types.VolumeBindings{
		// single normal request with too much size
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:2TiB",
		}),
		// multiple normal request with too much size
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:800GiB",
			"AUTO:/dir2:rw:800GiB",
			"AUTO:/dir3:rw:800GiB",
			"AUTO:/dir4:rw:800GiB",
			"AUTO:/dir5:rw:800GiB",
			"AUTO:/dir6:rw:800GiB",
			"AUTO:/dir7:rw:800GiB",
		}),
		// mono request with too much size
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rwm:500GiB",
			"AUTO:/dir2:rwm:500GiB",
			"AUTO:/dir3:rwm:500GiB",
		}),
		// insufficient unused volume
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rw:800GiB",
			"AUTO:/dir2:rw:800GiB",
			"AUTO:/dir3:rw:800GiB",
			"AUTO:/dir4:rw:800GiB",
			"AUTO:/dir5:rwm:500GiB",
			"AUTO:/dir6:rwm:500GiB",
		}),
	}

	for _, volumeRequest := range requests {
		resourceInfo = generateResourceInfo()
		plans = GetVolumePlans(resourceInfo, volumeRequest, maxDeployCount)
		assert.Equal(t, len(plans), 0)
	}
}

func generateExistingVolumePlan(t *testing.T) (types.VolumeBindings, types.VolumePlan) {
	plan := types.VolumePlan{}
	err := plan.UnmarshalJSON([]byte(`
{
	"AUTO:/dir0:rw:100GiB": {
        "/data0": 107374182400
      },
      "AUTO:/dir1:mrw:100GiB": {
        "/data2": 1099511627776
      },
      "AUTO:/dir2:rw:0": {
        "/data0": 0
      }
}
`))
	assert.Nil(t, err)
	bindings := generateVolumeBindings(t, []string{
		"AUTO:/dir0:rw:100GiB",
		"AUTO:/dir1:mrw:100GiB",
		"AUTO:/dir2:rw:0",
	})
	return bindings, plan
}

func TestGetAffinityPlan(t *testing.T) {
	// normal cases
	requests := []types.VolumeBindings{
		// realloc normal
		generateVolumeBindings(t, []string{
			"AUTO:/dir0:rw:1GiB",
		}),
		// realloc normal with reschedule
		generateVolumeBindings(t, []string{
			"AUTO:/dir3:rw:1GiB",
		}),
		// realloc mono without reschedule
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rwm:1GiB",
		}),
		// realloc mono without reschedule
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rwm:1GiB",
			"AUTO:/dir3:rwm:1GiB",
		}),
		// realloc mono with reschedule
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rwm:-1TiB",
			"AUTO:/dir3:rwm:100GiB",
			"AUTO:/dir4:rwm:100GiB",
		}),
		// realloc unlimited
		generateVolumeBindings(t, []string{
			"AUTO:/dir0:rw:-100GiB",
		}),
		// mixed
		generateVolumeBindings(t, []string{
			"AUTO:/dir0:rw:-100GiB",
			"AUTO:/dir2:rw:100GiB",
			"AUTO:/dir3:rwm:100GiB",
		}),
	}

	for _, request := range requests {
		resourceInfo := generateResourceInfo()
		originRequest, existing := generateExistingVolumePlan(t)
		for _, volumeMap := range existing {
			resourceInfo.Usage.Volumes[volumeMap.GetDevice()] += volumeMap.GetSize()
		}
		mergedRequest := types.MergeVolumeBindings(request, originRequest)

		plan := GetAffinityPlan(resourceInfo, mergedRequest, existing)
		validateVolumePlan(t, resourceInfo, mergedRequest, plan)
	}

	// no request
	resourceInfo := generateResourceInfo()
	originRequest, existing := generateExistingVolumePlan(t)
	for _, volumeMap := range existing {
		resourceInfo.Usage.Volumes[volumeMap.GetDevice()] += volumeMap.GetSize()
	}
	emptyRequest := types.VolumeBindings{}
	mergedRequest := types.MergeVolumeBindings(emptyRequest, originRequest)

	plan := GetAffinityPlan(resourceInfo, mergedRequest, existing)
	assert.Equal(t, existing.String(), plan.String())

	invalidRequests := []types.VolumeBindings{
		// normal request with too much size
		generateVolumeBindings(t, []string{
			"AUTO:/dir0:rw:1TiB",
		}),
		// mono request with too much size
		generateVolumeBindings(t, []string{
			"AUTO:/dir1:rwm:1TiB",
		}),
		// volumes are not enough
		generateVolumeBindings(t, []string{
			"AUTO:/dir3:rw:1TiB",
			"AUTO:/dir4:rw:1TiB",
		}),
	}

	for _, request := range invalidRequests {
		resourceInfo := generateResourceInfo()
		originRequest, existing := generateExistingVolumePlan(t)
		for _, volumeMap := range existing {
			resourceInfo.Usage.Volumes[volumeMap.GetDevice()] += volumeMap.GetSize()
		}
		mergedRequest := types.MergeVolumeBindings(request, originRequest)

		plan := GetAffinityPlan(resourceInfo, mergedRequest, existing)
		assert.Equal(t, len(plan), 0)
	}
}
