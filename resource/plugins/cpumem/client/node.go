package client

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
)

type cpuType interface {
	string | int64
}

// NodeResourceRequest includes all possible fields passed by eru-core for editing node, it not parsed!
type NodeResourceRequest[T cpuType] struct {
	CPU        T        `json:"cpu"`
	Share      int64    `json:"share"`
	Memory     int64    `json:"memory"`
	NUMA       []string `json:"numa-cpu"`
	NUMAMemory []int64  `json:"numa-memory"`

	NUMACPUMap    types.NUMA       `json:"-"`
	NUMAMemoryMap types.NUMAMemory `json:"-"`
}

// NewNodeResourceRequest .
func NewNodeResourceRequest[T cpuType](cpu T, share, memory int64, NUMACPUMap types.NUMA, NUMAMemoryMap types.NUMAMemory) (*NodeResourceRequest[T], error) {
	r := &NodeResourceRequest[T]{
		CPU:    cpu,
		Share:  share,
		Memory: memory,
	}
	tmpNUMA := map[int][]string{}
	for cpuID, nodeID := range NUMACPUMap {
		nID, err := strconv.Atoi(nodeID)
		if err != nil {
			return nil, err
		}
		if tmpNUMA[nID] == nil {
			tmpNUMA[nID] = []string{cpuID}
			continue
		}
		tmpNUMA[nID] = append(tmpNUMA[nID], cpuID)
	}
	r.NUMA = make([]string, len(tmpNUMA))
	for nodeID, cpus := range tmpNUMA {
		r.NUMA[nodeID] = strings.Join(cpus, ",")
	}

	tmpNUMAMemory := map[int]int64{}
	for nodeID, memory := range NUMAMemoryMap {
		nID, err := strconv.Atoi(nodeID)
		if err != nil {
			return nil, err
		}
		tmpNUMAMemory[nID] = memory
	}
	r.NUMAMemory = make([]int64, len(tmpNUMAMemory))
	for nodeID, memory := range tmpNUMAMemory {
		r.NUMAMemory[nodeID] = memory
	}

	return r, nil
}

// Encode .
func (n NodeResourceRequest[T]) Encode() ([]byte, error) {
	return json.Marshal(n)
}
