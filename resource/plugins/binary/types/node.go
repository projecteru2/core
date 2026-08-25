package types

import (
	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

type AddNodeRequest struct {
	Nodename string                   `json:"nodename" mapstructure:"nodename"`
	Resource plugintypes.NodeResource `json:"resource" mapstructure:"resource"`
	Info     *enginetypes.Info        `json:"info" mapstructure:"info"`
}

type RemoveNodeRequest struct {
	Nodename string `json:"nodename" mapstructure:"nodename"`
}

type GetNodesDeployCapacityRequest struct {
	Nodenames        []string                     `json:"nodenames" mapstructure:"nodenames"`
	WorkloadResource plugintypes.WorkloadResource `json:"workload_resource" mapstructure:"workload_resource"`
}

type SetNodeResourceCapacityRequest struct {
	Nodename        string                   `json:"nodename" mapstructure:"nodename"`
	Resource        plugintypes.NodeResource `json:"resource" mapstructure:"resource"`
	ResourceRequest plugintypes.NodeResource `json:"resource_request" mapstructure:"resource_request"`
	Delta           bool                     `json:"delta" mapstructure:"delta"`
	Incr            bool                     `json:"incr" mapstructure:"incr"`
}

type GetNodeResourceInfoRequest struct {
	Nodename          string                         `json:"nodename" mapstructure:"nodename"`
	WorkloadsResource []plugintypes.WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
}

type SetNodeResourceInfoRequest struct {
	Nodename string                   `json:"nodename" mapstructure:"nodename"`
	Capacity plugintypes.NodeResource `json:"capacity" mapstructure:"capacity"`
	Usage    plugintypes.NodeResource `json:"usage" mapstructure:"usage"`
}

type SetNodeResourceUsageRequest struct {
	Nodename          string                         `json:"nodename" mapstructure:"nodename"`
	WorkloadsResource []plugintypes.WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
	Resource          plugintypes.NodeResource       `json:"resource" mapstructure:"resource"`
	ResourceRequest   plugintypes.NodeResource       `json:"resource_request" mapstructure:"resource_request"`
	Delta             bool                           `json:"delta" mapstructure:"delta"`
	Incr              bool                           `json:"incr" mapstructure:"incr"`
}

type GetMostIdleNodeRequest struct {
	Nodenames []string `json:"nodenames" mapstructure:"nodenames"`
}
