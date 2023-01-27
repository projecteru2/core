package types

import (
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

// CalculateDeployRequest .
type CalculateDeployRequest struct {
	Nodename                string                               `json:"nodename" mapstructure:"nodename"`
	DeployCount             int                                  `json:"deploy_count" mapstructure:"deploy_count"`
	WorkloadResourceRequest *plugintypes.WorkloadResourceRequest `json:"workload_resource_request" mapstructure:"workload_resource_request"`
}

// CalculateReallocRequest .
type CalculateReallocRequest struct {
	Nodename                string                               `json:"nodename" mapstructure:"nodename"`
	WorkloadResource        *plugintypes.WorkloadResource        `json:"workload_request" mapstructure:"workload_request"`
	WorkloadResourceRequest *plugintypes.WorkloadResourceRequest `json:"workload_resource_request" mapstructure:"workload_resource_request"`
}

// CalculateRemapRequest .
type CalculateRemapRequest struct {
	Nodename          string                                   `json:"nodename" mapstructure:"nodename"`
	WorkloadsResource map[string]*plugintypes.WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
}
