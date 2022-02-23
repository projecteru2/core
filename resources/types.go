package resources

import "github.com/projecteru2/core/types"

// NodeCapacityInfo .
type NodeCapacityInfo struct {
	NodeName string
	Capacity int

	// Usage current resource usage
	Usage float64
	// Rate proportion of requested resources to total
	Rate float64

	// Weight used for weighted average
	Weight float64
}

type NodeResourceInfo struct {
	Capacity types.NodeResourceArgs `json:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage"`
}

// GetNodesDeployCapacityRequest .
type GetNodesDeployCapacityRequest struct {
	NodeNames    []string                   `json:"node"`
	ResourceOpts types.WorkloadResourceOpts `json:"resource-opts"`
}

// GetNodesDeployCapacityResponse .
type GetNodesDeployCapacityResponse struct {
	Nodes map[string]*NodeCapacityInfo `json:"nodes"`
	Total int                          `json:"total"`
}

// GetNodeResourceInfoRequest .
type GetNodeResourceInfoRequest struct {
	NodeName    string                                `json:"node"`
	WorkloadMap map[string]types.WorkloadResourceArgs `json:"workload-map"`
	Fix         bool                                  `json:"fix"`
}

// GetNodeResourceInfoResponse ,
type GetNodeResourceInfoResponse struct {
	ResourceInfo *NodeResourceInfo `json:"resource_info"`
	Diffs        []string          `json:"diffs"`
}

// SetNodeResourceInfoRequest .
type SetNodeResourceInfoRequest struct {
	NodeName string                 `json:"node"`
	Capacity types.NodeResourceArgs `json:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage"`
}

// SetNodeResourceInfoResponse .
type SetNodeResourceInfoResponse struct{}

// GetDeployArgsRequest .
type GetDeployArgsRequest struct {
	NodeName     string                     `json:"node"`
	DeployCount  int                        `json:"deploy"`
	ResourceOpts types.WorkloadResourceOpts `json:"resource-opts"`
}

// GetDeployArgsResponse .
type GetDeployArgsResponse struct {
	EngineArgs   []types.EngineArgs           `json:"engine_args"`
	ResourceArgs []types.WorkloadResourceArgs `json:"resource_args"`
}

// GetReallocArgsRequest .
type GetReallocArgsRequest struct {
	NodeName     string                     `json:"node"`
	Old          types.WorkloadResourceArgs `json:"old"`
	ResourceOpts types.WorkloadResourceOpts `json:"resource-opts"`
}

// GetReallocArgsResponse .
type GetReallocArgsResponse struct {
	EngineArgs   types.EngineArgs           `json:"engine_args"`
	Delta        types.WorkloadResourceArgs `json:"delta"`
	ResourceArgs types.WorkloadResourceArgs `json:"resource_args"`
}

// GetRemapArgsRequest .
type GetRemapArgsRequest struct {
	NodeName    string                                `json:"node"`
	WorkloadMap map[string]types.WorkloadResourceArgs `json:"workload-map"`
}

// GetRemapArgsResponse .
type GetRemapArgsResponse struct {
	EngineArgsMap map[string]types.EngineArgs `json:"engine_args_map"`
}

// SetNodeResourceUsageRequest .
type SetNodeResourceUsageRequest struct {
	NodeName             string                       `json:"node"`
	WorkloadResourceArgs []types.WorkloadResourceArgs `json:"workload-resource-args"`
	NodeResourceOpts     types.NodeResourceOpts       `json:"node-resource-opts"`
	NodeResourceArgs     types.NodeResourceArgs       `json:"node-resource-args"`
	Delta                bool                         `json:"delta"`
	Decr                 bool                         `json:"decr"`
}

// SetNodeResourceUsageResponse .
type SetNodeResourceUsageResponse struct {
	Before types.NodeResourceArgs `json:"before"`
	After  types.NodeResourceArgs `json:"after"`
}

// SetNodeResourceCapacityRequest .
type SetNodeResourceCapacityRequest struct {
	NodeName         string                 `json:"node"`
	NodeResourceOpts types.NodeResourceOpts `json:"node-resource-opts"`
	NodeResourceArgs types.NodeResourceArgs `json:"node-resource-args"`
	Delta            bool                   `json:"delta"`
	Decr             bool                   `json:"decr"`
}

// SetNodeResourceCapacityResponse .
type SetNodeResourceCapacityResponse struct {
	Before types.NodeResourceArgs `json:"before"`
	After  types.NodeResourceArgs `json:"after"`
}

// UpdateNodeResourceUsageRequest .
type UpdateNodeResourceUsageRequest struct {
	NodeName     string                       `json:"node"`
	ResourceArgs []types.WorkloadResourceArgs `json:"resource-args"`
	Decr         bool                         `json:"decr"`
}

// UpdateNodeResourceUsageResponse .
type UpdateNodeResourceUsageResponse struct{}

// UpdateNodeResourceCapacityRequest .
type UpdateNodeResourceCapacityRequest struct {
	NodeName     string                 `json:"node"`
	ResourceOpts types.NodeResourceOpts `json:"resource-opts"`
	Decr         bool                   `json:"decr"`
	Delta        bool                   `json:"delta"`
}

// UpdateNodeResourceCapacityResponse .
type UpdateNodeResourceCapacityResponse struct{}

// AddNodeRequest .
type AddNodeRequest struct {
	NodeName     string                 `json:"node"`
	ResourceOpts types.NodeResourceOpts `json:"resource-opts"`
}

// AddNodeResponse .
type AddNodeResponse struct {
	Capacity types.NodeResourceArgs `json:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage"`
}

// RemoveNodeRequest .
type RemoveNodeRequest struct {
	NodeName string `json:"node"`
}

// RemoveNodeResponse .
type RemoveNodeResponse struct{}

// GetMostIdleNodeRequest .
type GetMostIdleNodeRequest struct {
	NodeNames []string `json:"node"`
}

// GetMostIdleNodeResponse .
type GetMostIdleNodeResponse struct {
	NodeName string `json:"node"`
	Priority int    `json:"priority"`
}
