package types

// SelectAvailableNodesRequest .
type SelectAvailableNodesRequest struct {
	Nodes        []string              `json:"nodes"`
	ResourceOpts *WorkloadResourceOpts `json:"resource_opts"`
}

// AllocRequest .
type AllocRequest struct {
	Node         string                `json:"node"`
	DeployCount  string                `json:"deploy_count"`
	ResourceOpts *WorkloadResourceOpts `json:"resource_opts"`
}

// ReallocRequest .
type ReallocRequest struct {
	Node               string                `json:"node"`
	OriginResourceArgs *WorkloadResourceArgs `json:"origin_resource_args"`
	ResourceOpts       *WorkloadResourceOpts `json:"resource_opts"`
}

// RemapRequest .
type RemapRequest struct {
	Node                string                           `json:"node"`
	WorkloadResourceMap map[string]*WorkloadResourceArgs `json:"workload_resource_map"`
}

// UpdateNodeResourceUsageRequest .
type UpdateNodeResourceUsageRequest struct {
	Node         string            `json:"node"`
	ResourceArgs *NodeResourceArgs `json:"resource_args"`
	Increase     bool              `json:"increase"`
}

// UpdateNodeResourceCapacityRequest .
type UpdateNodeResourceCapacityRequest struct {
	Node         string            `json:"node"`
	ResourceOpts *NodeResourceOpts `json:"resource_opts"`
	Increase     bool              `json:"increase"`
}

// AddNodeRequest .
type AddNodeRequest struct {
	Node         string            `json:"node"`
	ResourceOpts *NodeResourceOpts `json:"resource_opts"`
}

// RemoveNodeRequest .
type RemoveNodeRequest struct {
	Node string `json:"node"`
}
