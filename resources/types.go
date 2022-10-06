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
	Capacity types.NodeResourceArgs `json:"capacity" mapstructure:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage" mapstructure:"usage"`
}

// GetNodesDeployCapacityRequest .
type GetNodesDeployCapacityRequest struct {
	NodeNames    []string                   `json:"nodenames" mapstructure:"nodenames"`
	ResourceOpts types.WorkloadResourceOpts `json:"resource_opts" mapstructure:"resource_opts"`
}

// GetNodesDeployCapacityResponse .
type GetNodesDeployCapacityResponse struct {
	Nodes map[string]*NodeCapacityInfo `json:"nodes" mapstructure:"nodes"`
	Total int                          `json:"total" mapstructure:"total"`
}

// GetNodeResourceInfoRequest .
type GetNodeResourceInfoRequest struct {
	NodeName    string                                `json:"nodename" mapstructure:"nodename"`
	WorkloadMap map[string]types.WorkloadResourceArgs `json:"workload_map" mapstructure:"workload_map"`
	Fix         bool                                  `json:"fix" mapstructure:"fix"`
}

// GetNodeResourceInfoResponse ,
type GetNodeResourceInfoResponse struct {
	ResourceInfo *NodeResourceInfo `json:"resource_info" mapstructure:"resource_info"`
	Diffs        []string          `json:"diffs" mapstructure:"diffs"`
}

// SetNodeResourceInfoRequest .
type SetNodeResourceInfoRequest struct {
	NodeName string                 `json:"node" mapstructure:"node"`
	Capacity types.NodeResourceArgs `json:"capacity" mapstructure:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage" mapstructure:"usage"`
}

// SetNodeResourceInfoResponse .
type SetNodeResourceInfoResponse struct{}

// GetDeployArgsRequest .
type GetDeployArgsRequest struct {
	NodeName     string                     `json:"nodename" mapstructure:"nodename"`
	DeployCount  int                        `json:"deploy_count" mapstructure:"deploy_count"`
	ResourceOpts types.WorkloadResourceOpts `json:"resource_opts" mapstructure:"resource_opts"`
}

// GetDeployArgsResponse .
type GetDeployArgsResponse struct {
	EngineArgs   []types.EngineArgs           `json:"engine_args" mapstructure:"engine_args"`
	ResourceArgs []types.WorkloadResourceArgs `json:"resource_args" mapstructure:"resource_args"`
}

// GetReallocArgsRequest .
type GetReallocArgsRequest struct {
	NodeName     string                     `json:"nodename" mapstructure:"nodename"`
	Old          types.WorkloadResourceArgs `json:"old" mapstructure:"old"`
	ResourceOpts types.WorkloadResourceOpts `json:"resource_opts" mapstructure:"resource_opts"`
}

// GetReallocArgsResponse .
type GetReallocArgsResponse struct {
	EngineArgs   types.EngineArgs           `json:"engine_args" mapstructure:"engine_args"`
	Delta        types.WorkloadResourceArgs `json:"delta" mapstructure:"delta"`
	ResourceArgs types.WorkloadResourceArgs `json:"resource_args" mapstructure:"resource_args"`
}

// GetRemapArgsRequest .
type GetRemapArgsRequest struct {
	NodeName    string                                `json:"nodename" mapstructure:"nodename"`
	WorkloadMap map[string]types.WorkloadResourceArgs `json:"workload_map" mapstructure:"workload_map"`
}

// GetRemapArgsResponse .
type GetRemapArgsResponse struct {
	EngineArgsMap map[string]types.EngineArgs `json:"engine_args_map" mapstructure:"engine_args_map"`
}

// SetNodeResourceUsageRequest .
type SetNodeResourceUsageRequest struct {
	NodeName             string                       `json:"nodename" mapstructure:"nodename"`
	WorkloadResourceArgs []types.WorkloadResourceArgs `json:"workload_resource_args" mapstructure:"workload_resource_args"`
	NodeResourceOpts     types.NodeResourceOpts       `json:"node_resource_opts" mapstructure:"node_resource_opts"`
	NodeResourceArgs     types.NodeResourceArgs       `json:"node_resource_args" mapstructure:"node_resource_args"`
	Delta                bool                         `json:"delta" mapstructure:"delta"`
	Decr                 bool                         `json:"decr" mapstructure:"decr"`
}

// SetNodeResourceUsageResponse .
type SetNodeResourceUsageResponse struct {
	Before types.NodeResourceArgs `json:"before" mapstructure:"before"`
	After  types.NodeResourceArgs `json:"after" mapstructure:"after"`
}

// SetNodeResourceCapacityRequest .
type SetNodeResourceCapacityRequest struct {
	NodeName         string                 `json:"nodename" mapstructure:"nodename"`
	NodeResourceOpts types.NodeResourceOpts `json:"node_resource_opts" mapstructure:"node_resource_opts"`
	NodeResourceArgs types.NodeResourceArgs `json:"node_resource_args" mapstructure:"node_resource_args"`
	Delta            bool                   `json:"delta" mapstructure:"delta"`
	Decr             bool                   `json:"decr" mapstructure:"decr"`
}

// SetNodeResourceCapacityResponse .
type SetNodeResourceCapacityResponse struct {
	Before types.NodeResourceArgs `json:"before" mapstructure:"before"`
	After  types.NodeResourceArgs `json:"after" mapstructure:"after"`
}

// AddNodeRequest .
type AddNodeRequest struct {
	NodeName     string                 `json:"nodename" mapstructure:"nodename"`
	ResourceOpts types.NodeResourceOpts `json:"resource_opts" mapstructure:"resource_opts"`
}

// AddNodeResponse .
type AddNodeResponse struct {
	Capacity types.NodeResourceArgs `json:"capacity" mapstructure:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage" mapstructure:"usage"`
}

// RemoveNodeRequest .
type RemoveNodeRequest struct {
	NodeName string `json:"nodename" mapstructure:"nodename"`
}

// RemoveNodeResponse .
type RemoveNodeResponse struct{}

// GetMostIdleNodeRequest .
type GetMostIdleNodeRequest struct {
	NodeNames []string `json:"nodenames" mapstructure:"nodenames"`
}

// GetMostIdleNodeResponse .
type GetMostIdleNodeResponse struct {
	NodeName string `json:"nodename"`
	Priority int    `json:"priority"`
}

// GetMetricsDescriptionRequest .
type GetMetricsDescriptionRequest struct{}

// MetricsDescription .
type MetricsDescription struct {
	Name   string   `json:"name" mapstructure:"name"`
	Help   string   `json:"help" mapstructure:"help"`
	Type   string   `json:"type" mapstructure:"type"`
	Labels []string `json:"labels" mapstructure:"labels"`
}

// GetMetricsDescriptionResponse .
type GetMetricsDescriptionResponse []*MetricsDescription

// GetNodeMetricsRequest .
type GetNodeMetricsRequest struct {
	PodName  string                 `json:"podname" mapstructure:"podname"`
	NodeName string                 `json:"nodename" mapstructure:"nodename"`
	Capacity types.NodeResourceArgs `json:"capacity" mapstructure:"capacity"`
	Usage    types.NodeResourceArgs `json:"usage" mapstructure:"usage"`
}

// Metrics .
type Metrics struct {
	Name   string   `json:"name" mapstructure:"name"`
	Labels []string `json:"labels" mapstructure:"labels"`
	Key    string   `json:"key" mapstructure:"key"`
	Value  string   `json:"value" mapstructure:"value"`
}

// GetNodeMetricsResponse .
type GetNodeMetricsResponse []*Metrics
