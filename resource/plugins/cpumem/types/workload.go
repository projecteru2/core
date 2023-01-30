package types

import (
	"github.com/cockroachdb/errors"
	"github.com/mitchellh/mapstructure"
	coretypes "github.com/projecteru2/core/types"
	coreutils "github.com/projecteru2/core/utils"
)

// WorkloadResource indicate cpumem workload resource
type WorkloadResource struct {
	CPURequest    float64    `json:"cpu_request" mapstructure:"cpu_request"`
	CPULimit      float64    `json:"cpu_limit" mapstructure:"cpu_limit"`
	MemoryRequest int64      `json:"memory_request" mapstructure:"memory_request"`
	MemoryLimit   int64      `json:"memory_limit" mapstructure:"memory_limit"`
	CPUMap        CPUMap     `json:"cpu_map" mapstructure:"cpu_map"`
	NUMAMemory    NUMAMemory `json:"numa_memory" mapstructure:"numa_memory"`
	NUMANode      string     `json:"numa_node" mapstructure:"numa_node"`
}

// ParseFromRawParams .
func (w *WorkloadResource) Parse(rawParams coretypes.RawParams) error {
	return mapstructure.Decode(rawParams, w)
}

// DeepCopy .
func (w *WorkloadResource) DeepCopy() *WorkloadResource {
	res := &WorkloadResource{
		CPURequest:    w.CPURequest,
		CPULimit:      w.CPULimit,
		MemoryRequest: w.MemoryRequest,
		MemoryLimit:   w.MemoryLimit,
		CPUMap:        CPUMap{},
		NUMAMemory:    NUMAMemory{},
		NUMANode:      w.NUMANode,
	}

	for cpu, pieces := range w.CPUMap {
		res.CPUMap[cpu] = pieces
	}
	for nodeID, nodeMemory := range res.NUMAMemory {
		res.NUMAMemory[nodeID] = nodeMemory
	}

	return res
}

// Add .
func (w *WorkloadResource) Add(w1 *WorkloadResource) {
	w.CPURequest = coreutils.Round(w.CPURequest + w1.CPURequest)
	w.MemoryRequest += w1.MemoryRequest
	w.CPUMap.Add(w1.CPUMap)

	if len(w.NUMAMemory) == 0 {
		w.NUMAMemory = w1.NUMAMemory
	} else {
		w.NUMAMemory.Add(w1.NUMAMemory)
	}
}

// Sub .
func (w *WorkloadResource) Sub(w1 *WorkloadResource) {
	w.CPURequest = coreutils.Round(w.CPURequest - w1.CPURequest)
	w.CPULimit = coreutils.Round(w.CPULimit - w1.CPULimit)
	w.MemoryRequest -= w1.MemoryRequest
	w.CPUMap.Sub(w1.CPUMap)
	if w.NUMAMemory == nil {
		w.NUMAMemory = NUMAMemory{}
	}
	w.NUMAMemory.Sub(w1.NUMAMemory)
}

// WorkloadResourceRaw includes all possible fields passed by eru-core for editing workload
// for request calculation
type WorkloadResourceRequest struct {
	CPUBind     bool    `json:"cpu_bind"`
	KeepCPUBind bool    `json:"keep_cpu_bind"`
	CPURequest  float64 `json:"cpu_request"`
	CPULimit    float64 `json:"cpu_limit"`
	MemRequest  int64   `json:"mem_request"`
	MemLimit    int64   `json:"mem_limit"`
}

// Validate .
func (w *WorkloadResourceRequest) Validate() error {
	if w.CPURequest == 0 && w.CPULimit > 0 {
		w.CPURequest = w.CPULimit
	}
	if w.MemLimit < 0 || w.MemRequest < 0 {
		return errors.Wrap(ErrInvalidMemory, "limit or request less than 0")
	}
	if w.CPURequest < 0 || w.CPULimit < 0 {
		return errors.Wrap(ErrInvalidCPU, "limit or request less than 0")
	}
	if w.CPURequest == 0 && w.CPUBind {
		return errors.Wrap(ErrInvalidCPU, "unlimited request with bind")
	}
	if w.MemRequest == 0 && w.MemLimit > 0 {
		w.MemRequest = w.MemLimit
	}
	if w.MemLimit > 0 && w.MemRequest > 0 && w.MemLimit < w.MemRequest {
		w.MemLimit = w.MemRequest
	}
	if w.CPURequest > 0 && w.CPULimit > 0 && w.CPULimit < w.CPURequest {
		w.CPULimit = w.CPURequest
	}
	// if CPUBind=true, set cpu request=limit to solve the dilemma
	// only deal with cpu limit>request but not vice versa
	if w.CPUBind && w.CPURequest > 0 && w.CPULimit > 0 && w.CPULimit > w.CPURequest {
		w.CPURequest = w.CPULimit
	}
	return nil
}

// Parse .
func (w *WorkloadResourceRequest) Parse(rawParams coretypes.RawParams) (err error) {
	w.KeepCPUBind = rawParams.Bool("keep-cpu-bind")
	w.CPUBind = rawParams.Bool("cpu-bind")

	w.CPURequest = rawParams.Float64("cpu-request")
	w.CPULimit = rawParams.Float64("cpu-limit")
	// check if cpu shortcut is set
	if cpu := rawParams.Float64("cpu"); cpu > 0 {
		w.CPURequest = cpu
		w.CPULimit = cpu
	}

	if w.MemRequest, err = coreutils.ParseRAMInHuman(rawParams.String("memory-request")); err != nil {
		return err
	}
	if w.MemLimit, err = coreutils.ParseRAMInHuman(rawParams.String("memory-limit")); err != nil {
		return err
	}
	// check if mem shortcut is set
	if rawParams.IsSet("memory") {
		var mem int64
		if mem, err = coreutils.ParseRAMInHuman(rawParams.String("memory")); err != nil {
			return err
		}
		w.MemLimit = mem
		w.MemRequest = mem
	}

	return nil
}
