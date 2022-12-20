package types

import (
	"github.com/projecteru2/core/types"
	coretypes "github.com/projecteru2/core/types"
)

// NodeResourceRequest use for raw data to create node
type NodeResourceRequest = coretypes.RawParams

// NodeResource use for indicate node's resource
type NodeResource = coretypes.RawParams

// AddNodeResponse .
type AddNodeResponse struct {
	Capacity *NodeResource `json:"capacity" mapstructure:"capacity"`
	Usage    *NodeResource `json:"usage" mapstructure:"usage"`
}

// RemoveNodeResponse .
type RemoveNodeResponse struct{}

// NodeDeployCapacity .
type NodeDeployCapacity struct {
	Capacity int
	// Usage current resource usage
	Usage float64
	// Rate proportion of requested resources to total
	Rate float64
	// Weight used for weighted average
	Weight float64
}

// GetNodesDeployCapacityResponse .
type GetNodesDeployCapacityResponse struct {
	NodeDeployCapacityMap map[string]*NodeDeployCapacity `json:"nodes_deploy_capacity_map" mapstructure:"nodes_deploy_capacity_map"`
	Total                 int                            `json:"total" mapstructure:"total"`
}

// SetNodeResourceCapacityResponse .
type SetNodeResourceCapacityResponse struct {
	Before *NodeResource `json:"before" mapstructure:"before"`
	After  *NodeResource `json:"after" mapstructure:"after"`
}

// GetNodeResourceInfoResponse ,
type GetNodeResourceInfoResponse struct {
	Capacity *NodeResource `json:"capacity" mapstructure:"capacity"`
	Usage    *NodeResource `json:"usage" mapstructure:"usage"`
	Diffs    []string      `json:"diffs" mapstructure:"diffs"`
}

// SetNodeResourceInfoResponse .
type SetNodeResourceInfoResponse struct{}

// SetNodeResourceUsageResponse .
type SetNodeResourceUsageResponse struct {
	Before *types.NodeResource `json:"before" mapstructure:"before"`
	After  *types.NodeResource `json:"after" mapstructure:"after"`
}

// GetMostIdleNodeResponse .
type GetMostIdleNodeResponse struct {
	Nodename string `json:"nodename"`
	Priority int    `json:"priority"`
}
