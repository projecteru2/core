package plugins

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

const (
	Incr = true
	Decr = false
)

type Plugin interface {
	// CalculateDeploy tries to allocate resource, returns engine params for each workload, format: [{"cpus": 1.2}, {"cpus": 1.2}]
	// also returns resource params for each workload, format: [{"cpus": 1.2}, {"cpus": 1.2}]
	// pure calculation
	CalculateDeploy(ctx context.Context, nodename string, deployCount int, resourceRequest plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateDeployResponse, error)

	// CalculateRealloc tries to reallocate resource, returns engine params, delta resource params and final resource params.
	// should return error if resource of some node is not enough for the realloc operation.
	// pure calculation
	CalculateRealloc(ctx context.Context, nodename string, resource plugintypes.WorkloadResource, resourceRequest plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateReallocResponse, error)

	// CalculateRemap tries to remap resource based on workload metadata and node resource usage, then returns engine params for workloads.
	// pure calculation; the map key is the workload ID
	CalculateRemap(ctx context.Context, nodename string, workloadsResource map[string]plugintypes.WorkloadResource) (*plugintypes.CalculateRemapResponse, error)

	// AddNode adds a node with requested resource, returns resource capacity and (empty) resource usage
	// should return error if the node already exists
	AddNode(ctx context.Context, nodename string, resource plugintypes.NodeResourceRequest, info *enginetypes.Info) (*plugintypes.AddNodeResponse, error)

	RemoveNode(ctx context.Context, nodename string) (*plugintypes.RemoveNodeResponse, error)

	GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource plugintypes.WorkloadResourceRequest) (*plugintypes.GetNodesDeployCapacityResponse, error)

	SetNodeResourceCapacity(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, delta, incr bool) (*plugintypes.SetNodeResourceCapacityResponse, error)

	// GetNodeResourceInfo returns total resource info and available resource info of the node, format: {"cpu": 2}
	// also returns diffs, format: ["node.VolumeUsed != sum(workload.VolumeRequest"]
	GetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error)

	// SetNodeResourceInfo sets both total node resource info and allocated resource info, used for rollback of RemoveNode
	// values are absolute, not deltas
	SetNodeResourceInfo(ctx context.Context, nodename string, capacity, usage plugintypes.NodeResource) (*plugintypes.SetNodeResourceInfoResponse, error)

	SetNodeResourceUsage(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, workloadsResource []plugintypes.WorkloadResource, delta, incr bool) (*plugintypes.SetNodeResourceUsageResponse, error)

	GetMostIdleNode(ctx context.Context, nodenames []string) (*plugintypes.GetMostIdleNodeResponse, error)

	FixNodeResource(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error)

	GetMetricsDescription(ctx context.Context) (*plugintypes.GetMetricsDescriptionResponse, error)

	GetMetrics(ctx context.Context, podname, nodename string) (*plugintypes.GetMetricsResponse, error)

	Name() string
}
