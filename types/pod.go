package types

// Pod define pod
type Pod struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// PodResource define pod resource
type PodResource struct {
	Name            string
	CPUPercents     map[string]float64
	MemoryPercents  map[string]float64
	StoragePercents map[string]float64
	VolumePercents  map[string]float64
	Verifications   map[string]bool
	Details         map[string]string
}
