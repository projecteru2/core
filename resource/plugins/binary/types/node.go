package types

import (
	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

type AddNodeRequest struct {
	Nodename string                   `json:"nodename"`
	Resource plugintypes.NodeResource `json:"resource"`
	Info     *enginetypes.Info        `json:"info"`
}

type RemoveNodeRequest struct {
	Nodename string `json:"nodename"`
}

type GetNodesDeployCapacityRequest struct {
	Nodenames        []string                     `json:"nodenames"`
	WorkloadResource plugintypes.WorkloadResource `json:"workload_resource"`
}

type SetNodeResourceCapacityRequest struct {
	Nodename        string                   `json:"nodename"`
	Resource        plugintypes.NodeResource `json:"resource"`
	ResourceRequest plugintypes.NodeResource `json:"resource_request"`
	Delta           bool                     `json:"delta"`
	Incr            bool                     `json:"incr"`
}

type GetNodeResourceInfoRequest struct {
	Nodename          string                         `json:"nodename"`
	WorkloadsResource []plugintypes.WorkloadResource `json:"workloads_resource"`
}

type SetNodeResourceInfoRequest struct {
	Nodename string                   `json:"nodename"`
	Capacity plugintypes.NodeResource `json:"capacity"`
	Usage    plugintypes.NodeResource `json:"usage"`
}

type SetNodeResourceUsageRequest struct {
	Nodename          string                         `json:"nodename"`
	WorkloadsResource []plugintypes.WorkloadResource `json:"workloads_resource"`
	Resource          plugintypes.NodeResource       `json:"resource"`
	ResourceRequest   plugintypes.NodeResource       `json:"resource_request"`
	Delta             bool                           `json:"delta"`
	Incr              bool                           `json:"incr"`
}

type GetMostIdleNodeRequest struct {
	Nodenames []string `json:"nodenames"`
}
