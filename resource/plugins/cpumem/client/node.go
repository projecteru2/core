package client

import (
	"encoding/json"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
)

type cpuType interface {
	string | int64
}

// NodeResourceRequest includes all possible fields passed by eru-core for editing node, it not parsed!
type NodeResourceRequest[T cpuType] struct {
	CPU    T     `json:"cpu"`
	Share  int64 `json:"share"`
	Memory int64 `json:"memory"`

	NUMA       types.NUMA       `json:"numa-cpu"`
	NUMAMemory types.NUMAMemory `json:"numa-memory"`
}

// NewNodeResourceRequest .
func NewNodeResourceRequest[T cpuType](cpu T, share, memory int64, NUMA types.NUMA, NUMAMemory types.NUMAMemory) (*NodeResourceRequest[T], error) {
	r := &NodeResourceRequest[T]{
		CPU:        cpu,
		Share:      share,
		Memory:     memory,
		NUMA:       NUMA,
		NUMAMemory: NUMAMemory,
	}
	return r, nil
}

// Encode .
func (n NodeResourceRequest[T]) Encode() ([]byte, error) {
	return json.Marshal(n)
}
