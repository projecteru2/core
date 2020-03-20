package types

// ResourceType .
type ResourceType int

const (
	// ResourceCPU .
	ResourceCPU ResourceType = 1 << iota
	// ResourceMemory .
	ResourceMemory
	// ResourceVolume .
	ResourceVolume
	// ResourceStorage .
	ResourceStorage
)

var (
	// ResourceAll .
	ResourceAll = ResourceStorage | ResourceMemory | ResourceCPU | ResourceVolume
	// AllResourceTypes .
	AllResourceTypes = [...]ResourceType{ResourceCPU, ResourceMemory, ResourceVolume, ResourceStorage}
)

// GetResourceType .
func GetResourceType(cpuBind, volumeSchedule bool) (resource ResourceType) {
	if cpuBind {
		resource |= ResourceCPU
	}
	if volumeSchedule {
		resource |= ResourceVolume
	}
	if resource == 0 {
		resource |= ResourceMemory
	}
	return
}
