package types

// Pod define pod
type Pod struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	// scheduler favor, should be CPU or MEM
	Favor string `json:"favor"`
}

// PodResource define pod resource
type PodResource struct {
	Name       string
	CPUPercent map[string]float64
	MEMPercent map[string]float64
	Diff       map[string]bool
	Detail     map[string]string
}
