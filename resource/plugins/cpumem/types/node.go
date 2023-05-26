package types

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/mitchellh/mapstructure"
	resourcetypes "github.com/projecteru2/core/resource/types"
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
func (r *NodeResource) Parse(rawParams resourcetypes.RawParams) error {
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
	if n.Capacity == nil {
		return ErrInvalidCapacity
	}

	if len(n.Capacity.CPUMap) == 0 {
		return ErrInvalidCPUMap
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

	for cpu, piecesUsed := range n.Usage.CPUMap {
		if totalPieces, ok := n.Capacity.CPUMap[cpu]; !ok || totalPieces < 0 || piecesUsed > totalPieces {
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
	CPUMap     CPUMap
	Memory     int64
	NUMA       NUMA
	NUMAMemory NUMAMemory
}

func (n *NodeResourceRequest) Parse(config coretypes.Config, rawParams resourcetypes.RawParams) error {
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
	n.Memory = rawParams.Int64("memory")

	n.NUMA = NUMA{}
	n.NUMAMemory = NUMAMemory{}
	for cpuID, nodeID := range rawParams.RawParams("numa-cpu") {
		n.NUMA[cpuID] = nodeID.(string)
	}

	for nodeID, nodeMemory := range rawParams.RawParams("numa-memory") {
		// stupid golang covert int64 to float64 when using json.unmarshal
		n.NUMAMemory[nodeID] = int64(nodeMemory.(float64))
	}
	return nil
}

func (n *NodeResourceRequest) LoadFromOrigin(nodeResource *NodeResource, resourceRequest resourcetypes.RawParams) {
	if n == nil {
		return
	}
	if !resourceRequest.IsSet("cpu") {
		n.CPUMap = nodeResource.CPUMap
	}
	if !resourceRequest.IsSet("memory") {
		n.Memory = nodeResource.Memory
	}
	if !resourceRequest.IsSet("numa-cpu") {
		n.NUMA = nodeResource.NUMA
	}
	if !resourceRequest.IsSet("numa-memory") {
		n.NUMAMemory = nodeResource.NUMAMemory
	}
}
