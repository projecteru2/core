package types

import "fmt"

// MustToVolumeBinding convert volume string into VolumeBinding or panic
func MustToVolumeBinding(volume string) VolumeBinding {
	vb, err := NewVolumeBinding(volume)
	if err != nil {
		panic(fmt.Errorf("invalid volume: %s", volume))
	}
	return *vb
}

// MustToVolumeBindings convert slice of volume string into VolumeBindings or panic
func MustToVolumeBindings(volumes []string) VolumeBindings {
	vbs, err := NewVolumeBindings(volumes)
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
