package types

type CPUPlan struct {
	NUMANode string
	CPUMap   CPUMap
}

type CPUMap map[string]int

func (c CPUMap) TotalPieces() int {
	res := 0
	for _, pieces := range c {
		res += pieces
	}
	return res
}

func (c CPUMap) Sub(c1 CPUMap) {
	for cpu, pieces := range c1 {
		c[cpu] -= pieces
	}
}

func (c CPUMap) Add(c1 CPUMap) {
	for cpu, pieces := range c1 {
		c[cpu] += pieces
	}
}

// NUMA map[cpuID]nodeID
type NUMA map[string]string

// NUMAMemory map[nodeID]memory
type NUMAMemory map[string]int64

func (n NUMAMemory) Add(n1 NUMAMemory) {
	for numaNodeID, memory := range n1 {
		n[numaNodeID] += memory
	}
}

func (n NUMAMemory) Sub(n1 NUMAMemory) {
	for numaNodeID, memory := range n1 {
		n[numaNodeID] -= memory
	}
}
