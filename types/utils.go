package types

import (
	"fmt"
)

func MustToVolumeBinding(volume string) VolumeBinding {
	vb, err := NewVolumeBinding(volume)
	if err != nil {
		panic(fmt.Errorf("invalid volume: %s", volume))
	}
	return *vb
}

func MustToVolumeBindings(volumes []string) VolumeBindings {
	vbs, err := MakeVolumeBindings(volumes)
	if err != nil {
		panic(fmt.Errorf("invalid volumes: %s", volumes))
	}
	return vbs
}

// MustToVolumePlan convert VolumePlan from literal value
func MustToVolumePlan(plan map[string]map[string]int64) VolumePlan {
	volumePlan := VolumePlan{}
	for volume, volumeMap := range plan {
		vb, err := NewVolumeBinding(volume)
		if err != nil {
			panic(fmt.Errorf("invalid plan %v: %v", plan, err))
		}
		volumePlan[*vb] = VolumeMap(volumeMap)
	}
	return volumePlan
}
