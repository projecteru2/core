package types

import (
	"maps"
	"strconv"
	"strings"

	"github.com/cockroachdb/errors"

	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// NodeResource is the cpu and memory of one node.
type NodeResource struct {
	CPU        float64    `json:"cpu"`
	CPUMap     CPUMap     `json:"cpu_map"`
	Memory     int64      `json:"memory"`
	NUMAMemory NUMAMemory `json:"numa_memory"`
	NUMA       NUMA       `json:"numa"`
}

func (r *NodeResource) Parse(rawParams resourcetypes.RawParams) error {
	return resourcetypes.Decode(rawParams, r)
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
	r.NUMAMemory.Add(r1.NUMAMemory)

	if len(r1.NUMA) > 0 {
		r.NUMA = r1.NUMA
	}
}

func (r *NodeResource) Sub(r1 *NodeResource) {
	r.CPU = coreutils.Round(r.CPU - r1.CPU)
	r.CPUMap.Sub(r1.CPUMap)
	r.Memory -= r1.Memory
	r.NUMAMemory.Sub(r1.NUMAMemory)
}

// NodeResourceInfo pairs a node's cpumem capacity with its usage.
type NodeResourceInfo struct {
	Capacity *NodeResource `json:"capacity"`
	Usage    *NodeResource `json:"usage"`
}

func (n *NodeResourceInfo) RemoveEmptyCores() {
	unused := func(cpu string, _ int) bool {
		return n.Capacity.CPUMap[cpu] == 0 && n.Usage.CPUMap[cpu] == 0
	}
	maps.DeleteFunc(n.Capacity.CPUMap, unused)
	maps.DeleteFunc(n.Usage.CPUMap, unused)

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

	// the stored record always carries objects, never nulls
	for _, r := range []*NodeResource{n.Capacity, n.Usage} {
		if r.CPUMap == nil {
			r.CPUMap = CPUMap{}
		}
		if r.NUMAMemory == nil {
			r.NUMAMemory = NUMAMemory{}
		}
		if r.NUMA == nil {
			r.NUMA = NUMA{}
		}
	}

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
			n.CPUMap[strconv.FormatInt(i, 10)] = int(share)
		}
	} else if cpuList := rawParams.String("cpu"); cpuList != "" {
		if err = n.parseCPUList(cpuList); err != nil {
			return err
		}
	}
	if n.Memory, err = rawParams.SizeInBytes("memory"); err != nil {
		return err
	}

	n.NUMA = NUMA{}
	n.NUMAMemory = NUMAMemory{}

	for index, numaCPUList := range rawParams.StringSlice("numa-cpu") {
		nodeID := strconv.Itoa(index)
		for cpuID := range strings.SplitSeq(numaCPUList, ",") {
			n.NUMA[cpuID] = nodeID
		}
	}

	for index, nodeMemory := range rawParams.StringSlice("numa-memory") {
		nodeID := strconv.Itoa(index)
		mem, err := resourcetypes.ParseRAMInHuman(nodeMemory)
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

func (n *NodeResourceRequest) parseCPUList(cpuList string) error {
	for cpus := range strings.SplitSeq(cpuList, ",") {
		cpuID, share, ok := strings.Cut(cpus, ":")
		if !ok {
			return errors.Wrapf(ErrInvalidCPUMap, "cpu: %s", cpus)
		}
		if _, err := strconv.Atoi(cpuID); err != nil {
			return err
		}
		pieces, err := strconv.ParseInt(share, 10, 32)
		if err != nil {
			return err
		}
		n.CPUMap[cpuID] = int(pieces)
	}
	return nil
}
