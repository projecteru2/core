package types

import (
	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

// AddNodeRequest .
type AddNodeRequest struct {
	Nodename string                    `json:"nodename" mapstructure:"nodename"`
	Resource *plugintypes.NodeResource `json:"resource" mapstructure:"resource"`
	Info     *enginetypes.Info         `json:"info" mapstructure:"info"`
}

// RemoveNodeRequest .
type RemoveNodeRequest struct {
	Nodename string `json:"nodename" mapstructure:"nodename"`
}

// GetNodesDeployCapacityRequest .
type GetNodesDeployCapacityRequest struct {
	Nodenames        []string                      `json:"nodenames" mapstructure:"nodenames"`
	WorkloadResource *plugintypes.WorkloadResource `json:"workload_resource" mapstructure:"workload_resource"`
}

// SetNodeResourceCapacityRequest .
type SetNodeResourceCapacityRequest struct {
	Nodename        string                    `json:"nodename" mapstructure:"nodename"`
	Resource        *plugintypes.NodeResource `json:"resource" mapstructure:"resource"`
	ResourceRequest *plugintypes.NodeResource `json:"resource_request" mapstructure:"resource_request"`
	Delta           bool                      `json:"delta" mapstructure:"delta"`
	Incr            bool                      `json:"incr" mapstructure:"incr"`
}

// GetNodeResourceInfoRequest .
type GetNodeResourceInfoRequest struct {
	Nodename          string                          `json:"nodename" mapstructure:"nodename"`
	WorkloadsResource []*plugintypes.WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
}

// SetNodeResourceInfoRequest .
type SetNodeResourceInfoRequest struct {
	Nodename string                    `json:"nodename" mapstructure:"nodename"`
	Capacity *plugintypes.NodeResource `json:"capacity" mapstructure:"capacity"`
	Usage    *plugintypes.NodeResource `json:"usage" mapstructure:"usage"`
}

// SetNodeResourceUsageRequest .
type SetNodeResourceUsageRequest struct {
	Nodename          string                          `json:"nodename" mapstructure:"nodename"`
	WorkloadsResource []*plugintypes.WorkloadResource `json:"workloads_resource" mapstructure:"workloads_resource"`
	Resource          *plugintypes.NodeResource       `json:"resource" mapstructure:"resource"`
	ResourceRequest   *plugintypes.NodeResource       `json:"resource_request" mapstructure:"resource_request"`
	Delta             bool                            `json:"delta" mapstructure:"delta"`
	Incr              bool                            `json:"incr" mapstructure:"incr"`
}

// GetMostIdleNodeRequest .
type GetMostIdleNodeRequest struct {
	Nodenames []string `json:"nodenames" mapstructure:"nodenames"`
}
