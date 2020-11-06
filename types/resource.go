package types

// Resource .
type Resource struct {
	CPUQuotaRequest float64
	CPUQuotaLimit   float64
	CPU             CPUMap
	CPUBind         bool // indicate CPU Bind
	NUMANode        string

	MemoryRequest int64
	MemoryLimit   int64

	VolumeRequest VolumeBindings
	VolumeLimit   VolumeBindings
	VolumeChanged bool // only for realloc used

	StorageRequest int64
	StorageLimit   int64
}

// ResourceType .
type ResourceType int

const (
	// ResourceCPU .
	ResourceCPU ResourceType = 1 << iota
	// ResourceCPUBind .
	ResourceCPUBind
	// ResourceMemory .
	ResourceMemory
	// ResourceVolume .
	ResourceVolume
	// ResourceScheduledVolume .
	ResourceScheduledVolume
	// ResourceStorage .
	ResourceStorage
)

var (
	// ResourceAll .
	ResourceAll = ResourceStorage | ResourceMemory | ResourceCPU | ResourceVolume
	// AllResourceTypes .
	AllResourceTypes = [...]ResourceType{ResourceCPU, ResourceMemory, ResourceVolume, ResourceStorage}
)

//// Resources .
//type Resources struct {
//	CPURequest      CPUMap
//	CPULimit        CPUMap
//	CPUQuotaRequest float64
//	CPUQuotaLimit   float64
//	CPUBind         bool
//	NUMANode        string
//
//	MemoryRequest   int64
//	MemoryLimit     int64
//	MemorySoftLimit bool
//
//	VolumeRequest     VolumeBindings
//	VolumeLimit       VolumeBindings
//	VolumePlanRequest VolumePlan
//	VolumePlanLimit   VolumePlan
//	VolumeChanged     bool
//
//	StorageRequest int64
//	StorageLimit   int64
//}
