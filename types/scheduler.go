package types

type ResourceType int

const (
	ResourceCPU ResourceType = 1 << iota
	ResourceMemory
	ResourceVolume
	ResourceStorage
)

var ResourceAll = ResourceStorage | ResourceMemory | ResourceCPU | ResourceVolume
var AllResourceTypes = [...]ResourceType{ResourceCPU, ResourceMemory, ResourceVolume, ResourceStorage}

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
