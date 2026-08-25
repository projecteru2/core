package types

import (
	"fmt"
	"maps"
	"strconv"
	"strings"

	"github.com/go-viper/mapstructure/v2"

	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// NodeResource is the cpu and memory of one node.
type NodeResource struct {
	CPU        float64    `json:"cpu" mapstructure:"cpu"`
	CPUMap     CPUMap     `json:"cpu_map" mapstructure:"cpu_map"`
	Memory     int64      `json:"memory" mapstructure:"memory"`
	NUMAMemory NUMAMemory `json:"numa_memory" mapstructure:"numa_memory"`
	NUMA       NUMA       `json:"numa" mapstructure:"numa"`
}

func (r *NodeResource) Parse(rawParams resourcetypes.RawParams) error {
	return mapstructure.Decode(rawParams, r)
}

func (r *NodeResource) DeepCopy() *NodeResource {
	res := &NodeResource{
		CPU:        r.CPU,
		CPUMap:     CPUMap{},
		Memory:     r.Memory,
		NUMAMemory: NUMAMemory{},
		NUMA:       NUMA{},
	}

	maps.Copy(res.CPUMap, r.CPUMap)
	maps.Copy(res.NUMAMemory, r.NUMAMemory)
	maps.Copy(res.NUMA, r.NUMA)
	return res
}

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

func (r *NodeResource) Sub(r1 *NodeResource) {
	r.CPU = coreutils.Round(r.CPU - r1.CPU)
	r.CPUMap.Sub(r1.CPUMap)
	r.Memory -= r1.Memory

	for numaNodeID := range r1.NUMAMemory {
		r.NUMAMemory[numaNodeID] -= r1.NUMAMemory[numaNodeID]
	}
}

// NodeResourceInfo pairs a node's cpumem capacity with its usage.
type NodeResourceInfo struct {
	Capacity *NodeResource `json:"capacity"`
	Usage    *NodeResource `json:"usage"`
}

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
		maps.Copy(n.Usage.NUMA, n.Capacity.NUMA)
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

	// DeepCopy replaces the nil CPUMap, NUMA and NUMAMemory with empty ones
	n.Capacity = n.Capacity.DeepCopy()
	n.Usage = n.Usage.DeepCopy()

	return nil
}

func (n *NodeResourceInfo) GetAvailableResource() *NodeResource {
	availableResource := n.Capacity.DeepCopy()
	availableResource.Sub(n.Usage)

	return availableResource
}

// NodeResourceRequest carries every raw field eru-core may pass when editing a node.
type NodeResourceRequest struct {
	CPUMap     CPUMap
	Memory     int64
	NUMA       NUMA
	NUMAMemory NUMAMemory
}

func (n *NodeResourceRequest) Parse(config coretypes.Config, rawParams resourcetypes.RawParams) error {
	var err error

	if n.CPUMap == nil {
		n.CPUMap = CPUMap{}
	}

	if cpu := rawParams.Int64("cpu"); cpu > 0 {
		share := rawParams.Int64("share")
		if share == 0 {
			share = int64(config.Scheduler.ShareBase)
		}

		for i := range cpu {
			n.CPUMap[fmt.Sprintf("%+v", i)] = int(share)
		}
	} else if cpuList := rawParams.String("cpu"); cpuList != "" {
		for cpus := range strings.SplitSeq(cpuList, ",") {
			cpuConfigs := strings.Split(cpus, ":")
			pieces, parseErr := strconv.ParseInt(cpuConfigs[1], 10, 32)
			if parseErr != nil {
				return parseErr
			}
			cpuID := cpuConfigs[0]
			if _, idErr := strconv.Atoi(cpuID); idErr != nil {
				return idErr
			}
			n.CPUMap[cpuID] = int(pieces)
		}
	}
	if mem := rawParams.Int64("memory"); mem > 0 {
		n.Memory = mem
	} else if n.Memory, err = coreutils.ParseRAMInHuman(rawParams.String("memory")); err != nil {
		return err
	}

	n.NUMA = NUMA{}
	n.NUMAMemory = NUMAMemory{}

	for index, numaCPUList := range rawParams.StringSlice("numa-cpu") {
		nodeID := fmt.Sprintf("%d", index)
		for cpuID := range strings.SplitSeq(numaCPUList, ",") {
			n.NUMA[cpuID] = nodeID
		}
	}

	for index, nodeMemory := range rawParams.StringSlice("numa-memory") {
		nodeID := fmt.Sprintf("%d", index)
		mem, err := coreutils.ParseRAMInHuman(nodeMemory)
		if err != nil {
			return err
		}
		n.NUMAMemory[nodeID] = mem
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
