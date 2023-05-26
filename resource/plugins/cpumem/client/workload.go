package client

import "encoding/json"

// WorkloadResourceRequest .
type WorkloadResourceRequest struct {
	CPUBind     bool    `json:"cpu-bind"`
	KeepCPUBind bool    `json:"keep-cpu-bind"`
	CPURequest  float64 `json:"cpu-request"`
	CPULimit    float64 `json:"cpu-limit"`
	MemRequest  int64   `json:"mem-request"`
	MemLimit    int64   `json:"mem-limit"`
}

// NewWorkloadResourceRequest .
func NewWorkloadResourceRequest(cpuBind, keepCPUBind bool, CPURequest, CPULimit float64, MemRequest, MemLimit int64) (*WorkloadResourceRequest, error) {
	return &WorkloadResourceRequest{
		CPUBind:     cpuBind,
		KeepCPUBind: keepCPUBind,
		CPURequest:  CPURequest,
		CPULimit:    CPULimit,
		MemRequest:  MemRequest,
		MemLimit:    MemLimit,
	}, nil
}

// Encode .
func (w WorkloadResourceRequest) Encode() ([]byte, error) {
	return json.Marshal(w)
}
