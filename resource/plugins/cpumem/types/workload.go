package types

import (
	"maps"

	"github.com/cockroachdb/errors"

	resourcetypes "github.com/projecteru2/core/resource/types"
	coreutils "github.com/projecteru2/core/utils"
)

// WorkloadResource is the cpu and memory allocated to one workload.
type WorkloadResource struct {
	CPURequest    float64    `json:"cpu_request"`
	CPULimit      float64    `json:"cpu_limit"`
	MemoryRequest int64      `json:"memory_request"`
	MemoryLimit   int64      `json:"memory_limit"`
	CPUMap        CPUMap     `json:"cpu_map"`
	NUMAMemory    NUMAMemory `json:"numa_memory"`
	NUMANode      string     `json:"numa_node"`
}

func (w *WorkloadResource) Parse(rawParams resourcetypes.RawParams) error {
	return resourcetypes.Decode(rawParams, w)
}

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

	maps.Copy(res.CPUMap, w.CPUMap)
	maps.Copy(res.NUMAMemory, w.NUMAMemory)

	return res
}

func (w *WorkloadResource) Add(w1 *WorkloadResource) {
	w.CPURequest = coreutils.Round(w.CPURequest + w1.CPURequest)
	w.MemoryRequest += w1.MemoryRequest
	w.CPUMap.Add(w1.CPUMap)

	if w.NUMAMemory == nil {
		w.NUMAMemory = NUMAMemory{}
	}
	w.NUMAMemory.Add(w1.NUMAMemory)
}

func (w *WorkloadResource) Sub(w1 *WorkloadResource) {
	w.CPURequest = coreutils.Round(w.CPURequest - w1.CPURequest)
	w.CPULimit = coreutils.Round(w.CPULimit - w1.CPULimit)
	w.MemoryRequest -= w1.MemoryRequest
	w.MemoryLimit -= w1.MemoryLimit
	w.CPUMap.Sub(w1.CPUMap)
	if w.NUMAMemory == nil {
		w.NUMAMemory = NUMAMemory{}
	}
	w.NUMAMemory.Sub(w1.NUMAMemory)
}

// WorkloadResourceRequest carries every field eru-core may pass when editing a workload.
type WorkloadResourceRequest struct {
	CPUBind     bool
	KeepCPUBind bool
	CPURequest  float64
	CPULimit    float64
	MemRequest  int64
	MemLimit    int64
}

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
	// a cpu-bound workload gets request raised to limit, never the other way round
	if w.CPUBind && w.CPURequest > 0 && w.CPULimit > 0 && w.CPULimit > w.CPURequest {
		w.CPURequest = w.CPULimit
	}
	return nil
}

func (w *WorkloadResourceRequest) Parse(rawParams resourcetypes.RawParams) (err error) {
	w.KeepCPUBind = rawParams.Bool("keep-cpu-bind")
	w.CPUBind = rawParams.Bool("cpu-bind")

	w.CPURequest = rawParams.Float64("cpu-request")
	w.CPULimit = rawParams.Float64("cpu-limit")

	if w.MemRequest, err = rawParams.SizeInBytes("memory-request"); err != nil {
		return err
	}
	if w.MemLimit, err = rawParams.SizeInBytes("memory-limit"); err != nil {
		return err
	}
	return nil
}
