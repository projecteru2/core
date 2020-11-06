package types

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

// DeployInfo .
type DeployInfo struct {
	Deploy int
}
