package types

// CPUPlan .
type CPUPlan struct {
	NUMANode string
	CPUMap   CPUMap
}

// CPUMap .
type CPUMap map[string]int

// TotalPieces .
func (c CPUMap) TotalPieces() int {
	res := 0
	for _, pieces := range c {
		res += pieces
	}
	return res
}

// Sub .
func (c CPUMap) Sub(c1 CPUMap) {
	for cpu, pieces := range c1 {
		c[cpu] -= pieces
	}
}

// Add .
func (c CPUMap) Add(c1 CPUMap) {
	for cpu, pieces := range c1 {
		c[cpu] += pieces
	}
}

// NUMA map[cpuID]nodeID
type NUMA map[string]string

// NUMAMemory .
type NUMAMemory map[string]int64

// Add .
func (n NUMAMemory) Add(n1 NUMAMemory) {
	for numaNodeID, memory := range n1 {
		n[numaNodeID] += memory
	}
}

// Sub .
func (n NUMAMemory) Sub(n1 NUMAMemory) {
	for numaNodeID, memory := range n1 {
		n[numaNodeID] -= memory
	}
}
