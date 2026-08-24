package types

import (
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

type CalculateDeployRequest struct {
	Nodename                string                              `json:"nodename" mapstructure:"nodename"`
	DeployCount             int                                 `json:"deploy_count" mapstructure:"deploy_count"`
	WorkloadResourceRequest plugintypes.WorkloadResourceRequest `json:"workload_resource_request" mapstructure:"workload_resource_request"`
}

type CalculateReallocRequest struct {
	Nodename                string                              `json:"nodename" mapstructure:"nodename"`
	WorkloadResource        plugintypes.WorkloadResource        `json:"workload_resource" mapstructure:"workload_resource"`
	WorkloadResourceRequest plugintypes.WorkloadResourceRequest `json:"workload_resource_request" mapstructure:"workload_resource_request"`
}

type CalculateRemapRequest struct {
	Nodename          string                                  `json:"nodename" mapstructure:"nodename"`
	WorkloadsResource map[string]plugintypes.WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
}
