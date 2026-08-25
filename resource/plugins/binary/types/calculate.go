package types

import (
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

type CalculateDeployRequest struct {
	Nodename                string                              `json:"nodename"`
	DeployCount             int                                 `json:"deploy_count"`
	WorkloadResourceRequest plugintypes.WorkloadResourceRequest `json:"workload_resource_request"`
}

type CalculateReallocRequest struct {
	Nodename                string                              `json:"nodename"`
	WorkloadResource        plugintypes.WorkloadResource        `json:"workload_resource"`
	WorkloadResourceRequest plugintypes.WorkloadResourceRequest `json:"workload_resource_request"`
}

type CalculateRemapRequest struct {
	Nodename          string                                  `json:"nodename"`
	WorkloadsResource map[string]plugintypes.WorkloadResource `json:"workloads_resource"`
}
