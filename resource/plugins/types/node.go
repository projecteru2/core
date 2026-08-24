package types

import (
	resourcetypes "github.com/projecteru2/core/resource/types"
)

type NodeResourceRequest = resourcetypes.RawParams

type NodeResource = resourcetypes.RawParams

type AddNodeResponse struct {
	Capacity NodeResource `json:"capacity" mapstructure:"capacity"`
	Usage    NodeResource `json:"usage" mapstructure:"usage"`
}

type RemoveNodeResponse struct{}

type NodeDeployCapacity struct {
	Capacity int
	// Usage is the used fraction of the node's total, 0..1
	Usage float64
	// Rate proportion of requested resources to total
	Rate float64
	// Weight used for weighted average
	Weight float64
}

type GetNodesDeployCapacityResponse struct {
	NodeDeployCapacityMap map[string]*NodeDeployCapacity `json:"nodes_deploy_capacity_map" mapstructure:"nodes_deploy_capacity_map"`
	Total                 int                            `json:"total" mapstructure:"total"`
}

type SetNodeResourceCapacityResponse struct {
	Before NodeResource `json:"before" mapstructure:"before"`
	After  NodeResource `json:"after" mapstructure:"after"`
}

type GetNodeResourceInfoResponse struct {
	Capacity NodeResource `json:"capacity" mapstructure:"capacity"`
	Usage    NodeResource `json:"usage" mapstructure:"usage"`
	Diffs    []string     `json:"diffs" mapstructure:"diffs"`
}

type SetNodeResourceInfoResponse struct{}

type SetNodeResourceUsageResponse struct {
	Before NodeResource `json:"before" mapstructure:"before"`
	After  NodeResource `json:"after" mapstructure:"after"`
}

type GetMostIdleNodeResponse struct {
	Nodename string `json:"nodename"`
	Priority int    `json:"priority"`
}
