package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// NodeResource indicate node cpumem resource
type NodeResource struct {
	CPU        float64    `json:"cpu" mapstructure:"cpu"`
	CPUMap     CPUMap     `json:"cpu_map" mapstructure:"cpu_map"`
	Memory     int64      `json:"memory" mapstructure:"memory"`
	NUMAMemory NUMAMemory `json:"numa_memory" mapstructure:"numa_memory"`
	NUMA       NUMA       `json:"numa" mapstructure:"numa"`
}

// Parse .
func (r *NodeResource) Parse(rawParams *coretypes.RawParams) error {
	return mapstructure.Decode(rawParams, r)
}

// DeepCopy .
func (r *NodeResource) DeepCopy() *NodeResource {
	res := &NodeResource{
		CPU:        r.CPU,
		CPUMap:     CPUMap{},
		Memory:     r.Memory,
		NUMAMemory: NUMAMemory{},
		NUMA:       NUMA{},
	}

	for cpu := range r.CPUMap {
		res.CPUMap[cpu] = r.CPUMap[cpu]
	}
	for numaNodeID := range r.NUMAMemory {
		res.NUMAMemory[numaNodeID] = r.NUMAMemory[numaNodeID]
	}
	for cpuID := range r.NUMA {
		res.NUMA[cpuID] = r.NUMA[cpuID]
	}
	return res
}

// Add .
func (r *NodeResource) Add(r1 *NodeResource) {
	r.CPU = coreutils.Round(r.CPU + r1.CPU)
	r.CPUMap.Add(r1.CPUMap)
	r.Memory += r1.Memory

	for numaNodeID := range r1.NUMAMemory {
		r.NUMAMemory[numaNodeID] += r1.NUMAMemory[numaNodeID]
	}

	if len(r1.NUMA) > 0 {
		r.NUMA = r1.NUMA
	}
}

// Sub .
func (r *NodeResource) Sub(r1 *NodeResource) {
	r.CPU = coreutils.Round(r.CPU - r1.CPU)
	r.CPUMap.Sub(r1.CPUMap)
	r.Memory -= r1.Memory

	for numaNodeID := range r1.NUMAMemory {
		r.NUMAMemory[numaNodeID] -= r1.NUMAMemory[numaNodeID]
	}
}

// NodeResourceInfo indicate cpumem capacity and usage
type NodeResourceInfo struct {
	Capacity *NodeResource `json:"capacity"`
	Usage    *NodeResource `json:"usage"`
}

// DeepCopy .
func (n *NodeResourceInfo) DeepCopy() *NodeResourceInfo {
	return &NodeResourceInfo{
		Capacity: n.Capacity.DeepCopy(),
		Usage:    n.Usage.DeepCopy(),
	}
}

// RemoveEmptyCores .
func (n *NodeResourceInfo) RemoveEmptyCores() {
	for cpu := range n.Capacity.CPUMap {
		if n.Capacity.CPUMap[cpu] == 0 && n.Usage.CPUMap[cpu] == 0 {
			delete(n.Capacity.CPUMap, cpu)
		}
	}
	for cpu := range n.Usage.CPUMap {
		if n.Capacity.CPUMap[cpu] == 0 && n.Usage.CPUMap[cpu] == 0 {
			delete(n.Usage.CPUMap, cpu)
		}
	}

	n.Capacity.CPU = float64(len(n.Capacity.CPUMap))
}

func (n *NodeResourceInfo) Validate() error {
	if n.Capacity == nil || len(n.Capacity.CPUMap) == 0 {
		return ErrInvalidCapacity
	}
	if n.Usage == nil {
		n.Usage = &NodeResource{
			CPU:        0,
			CPUMap:     CPUMap{},
			Memory:     0,
			NUMAMemory: NUMAMemory{},
			NUMA:       NUMA{},
		}
		for cpuID := range n.Capacity.CPUMap {
			n.Usage.CPUMap[cpuID] = 0
		}
		for numaNodeID := range n.Capacity.NUMAMemory {
			n.Usage.NUMAMemory[numaNodeID] = 0
		}
		for cpuID, numaNodeID := range n.Capacity.NUMA {
			n.Usage.NUMA[cpuID] = numaNodeID
		}
	}
	if len(n.Capacity.CPUMap) == 0 {
		return ErrInvalidCPUMap
	}

	for cpu, piecesUsed := range n.Usage.CPUMap {
		if totalPieces, ok := n.Capacity.CPUMap[cpu]; !ok || piecesUsed < 0 || totalPieces < 0 || piecesUsed > totalPieces {
			return ErrInvalidCPUMap
		}
	}

	if len(n.Capacity.NUMA) > 0 {
		for cpu := range n.Capacity.CPUMap {
			if numaNodeID, ok := n.Capacity.NUMA[cpu]; !ok {
				return ErrInvalidNUMACPU
			} else if _, ok = n.Capacity.NUMAMemory[numaNodeID]; !ok {
				return ErrInvalidNUMAMemory
			}
		}

		for numaNodeID, nodeMemory := range n.Capacity.NUMAMemory {
			if nodeMemory < 0 {
				return ErrInvalidNUMAMemory
			}
			if memoryUsed := n.Usage.NUMAMemory[numaNodeID]; memoryUsed < 0 || memoryUsed > nodeMemory {
				return ErrInvalidNUMAMemory
			}
		}
	}

	// remove nil CPUMap / NUMA / NUMAMemory
	n.Capacity = n.Capacity.DeepCopy()
	n.Usage = n.Usage.DeepCopy()

	return nil
}

func (n *NodeResourceInfo) GetAvailableResource() *NodeResource {
	availableResource := n.Capacity.DeepCopy()
	availableResource.Sub(n.Usage)

	return availableResource
}

// NodeResourceRequest includes all possible fields passed by eru-core for editing node, it not parsed!
type NodeResourceRequest struct {
	CPUMap     CPUMap     `json:"cpu_map"`
	Memory     int64      `json:"memory"`
	NUMA       NUMA       `json:"numa"`
	NUMAMemory NUMAMemory `json:"numa_memory"`
}

func (n *NodeResourceRequest) Parse(config coretypes.Config, rawParams *coretypes.RawParams) error {
	var err error

	if n.CPUMap == nil {
		n.CPUMap = CPUMap{}
	}

	if cpu := rawParams.Int64("cpu"); cpu > 0 { //nolint
		share := rawParams.Int64("share")
		if share == 0 {
			share = int64(config.Scheduler.ShareBase)
		}

		for i := int64(0); i < cpu; i++ {
			n.CPUMap[fmt.Sprintf("%+v", i)] = int(share)
		}
	} else if cpuList := rawParams.String("cpu"); cpuList != "" {
		for _, cpus := range strings.Split(cpuList, ",") {
			cpuConfigs := strings.Split(cpus, ":")
			pieces, err := strconv.ParseInt(cpuConfigs[1], 10, 32)
			if err != nil {
				return err
			}
			cpuID := cpuConfigs[0]
			if _, err := strconv.Atoi(cpuID); err != nil {
				return err
			}
			n.CPUMap[cpuID] = int(pieces)
		}
	}

	if n.Memory, err = coreutils.ParseRAMInHuman(rawParams.String("memory")); err != nil {
		return err
	}

	n.NUMA = NUMA{}
	n.NUMAMemory = NUMAMemory{}

	for index, numaCPUList := range rawParams.StringSlice("numa-cpu") {
		nodeID := fmt.Sprintf("%d", index)
		for _, cpuID := range strings.Split(numaCPUList, ",") {
			n.NUMA[cpuID] = nodeID
		}
	}

	for index, numaMemoryNode := range rawParams.StringSlice("numa-memory") {
		nodeID := fmt.Sprintf("%d", index)
		mem, err := coreutils.ParseRAMInHuman(numaMemoryNode)
		if err != nil {
			return err
		}
		n.NUMAMemory[nodeID] = mem
	}

	return nil
}

func (n *NodeResourceRequest) LoadFromNodeResource(nodeResource *NodeResource, rawParams *coretypes.RawParams) {
	if n == nil {
		return
	}
	if !rawParams.IsSet("cpu") {
		n.CPUMap = nodeResource.CPUMap
	}
	if !rawParams.IsSet("memory") {
		n.Memory = nodeResource.Memory
	}
	if !rawParams.IsSet("numa-cpu") {
		n.NUMA = nodeResource.NUMA
	}
	if !rawParams.IsSet("numa-memory") {
		n.NUMAMemory = nodeResource.NUMAMemory
	}
}
