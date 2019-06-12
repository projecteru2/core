package types

// Pod define pod
type Pod struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
}

// PodResource define pod resource
type PodResource struct {
	Name          string
	CPUPercent    map[string]float64
	MEMPercent    map[string]float64
	Verifications map[string]bool
	Details       map[string]string
}
