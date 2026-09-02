package types

import (
	resourcetypes "github.com/projecteru2/core/resource/types"
)

type NodeResourceRequest = resourcetypes.RawParams

type NodeResource = resourcetypes.RawParams

type AddNodeResponse struct {
	Capacity NodeResource `json:"capacity"`
	Usage    NodeResource `json:"usage"`
}

type RemoveNodeResponse struct{}

type NodeDeployCapacity struct {
	Capacity int `json:"capacity"`
	// Usage is the used fraction of the node's total, 0..1
	Usage float64 `json:"usage"`
	// Rate proportion of requested resources to total
	Rate float64 `json:"rate"`
	// Weight used for weighted average
	Weight float64 `json:"weight"`
}

type GetNodesDeployCapacityResponse struct {
	NodeDeployCapacityMap map[string]*NodeDeployCapacity `json:"nodes_deploy_capacity_map"`
	Total                 int                            `json:"total"`
}

type SetNodeResourceCapacityResponse struct {
	Before NodeResource `json:"before"`
	After  NodeResource `json:"after"`
}

type GetNodeResourceInfoResponse struct {
	Capacity NodeResource `json:"capacity"`
	Usage    NodeResource `json:"usage"`
	Diffs    []string     `json:"diffs"`
}

type SetNodeResourceInfoResponse struct{}

type SetNodeResourceUsageResponse struct {
	Before NodeResource `json:"before"`
	After  NodeResource `json:"after"`
}

type GetMostIdleNodeResponse struct {
	Nodename string `json:"nodename"`
	Priority int    `json:"priority"`
}
