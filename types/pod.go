package types

const (
	CPU_PRIOR = "CPU"
	MEMORY_PRIOR = "MEM"
)

type Pod struct {
	Name string `json:"name"`
	Desc string `json:"desc"`
	// scheduler favor, should be CPU or MEM
	Favor string `json:"favor"`
}
