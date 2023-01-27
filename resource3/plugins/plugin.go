package plugins

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
)

const (
	// Incr increase
	Incr = true

	// Decr decrease
	Decr = false
)

// Plugin .
type Plugin interface {
	// CalculateDeploy tries to allocate resource, returns engine params for each workload, format: [{"cpus": 1.2}, {"cpus": 1.2}]
	// also returns resource params for each workload, format: [{"cpus": 1.2}, {"cpus": 1.2}]
	// pure calculation
	CalculateDeploy(ctx context.Context, nodename string, deployCount int, resourceRequest *plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateDeployResponse, error)

	// CalculateRealloc tries to reallocate resource, returns engine params, delta resource params and final resource params.
	// should return error if resource of some node is not enough for the realloc operation.
	// pure calculation
	CalculateRealloc(ctx context.Context, nodename string, resource *plugintypes.WorkloadResource, resourceRequest *plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateReallocResponse, error)

	// CalculateRemap tries to remap resource based on workload metadata and node resource usage, then returns engine params for workloads.
	// pure calculation, for clarification, here use map's key to store the workload ID
	CalculateRemap(ctx context.Context, nodename string, workloadsResource map[string]*plugintypes.WorkloadResource) (*plugintypes.CalculateRemapResponse, error)

	// AddNode adds a node with requested resource, returns resource capacity and (empty) resource usage
	// should return error if the node already exists
	AddNode(ctx context.Context, nodename string, resource *plugintypes.NodeResourceRequest, info *enginetypes.Info) (*plugintypes.AddNodeResponse, error)

	// RemoveNode removes node
	RemoveNode(ctx context.Context, nodename string) (*plugintypes.RemoveNodeResponse, error)

	// GetNodesDeployCapacity returns available nodes and total capacity
	GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource *plugintypes.WorkloadResourceRequest) (*plugintypes.GetNodesDeployCapacityResponse, error)

	// SetNodeResourceCapacity sets the amount of total resource info
	SetNodeResourceCapacity(ctx context.Context, nodename string, resource *plugintypes.NodeResource, resourceRequest *plugintypes.NodeResourceRequest, delta bool, incr bool) (*plugintypes.SetNodeResourceCapacityResponse, error)

	// GetNodeResourceInfo returns total resource info and available resource info of the node, format: {"cpu": 2}
	// also returns diffs, format: ["node.VolumeUsed != sum(workload.VolumeRequest"]
	GetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error)

	// SetNodeResourceInfo sets both total node resource info and allocated resource info
	// used for rollback of RemoveNode
	// notice: here uses absolute values, not delta values
	SetNodeResourceInfo(ctx context.Context, nodename string, capacity *plugintypes.NodeResource, usage *plugintypes.NodeResource) (*plugintypes.SetNodeResourceInfoResponse, error)

	// SetNodeResourceUsage sets the amount of allocated resource info
	SetNodeResourceUsage(ctx context.Context, nodename string, resource *plugintypes.NodeResource, resourceRequest *plugintypes.NodeResourceRequest, workloadsResource []*plugintypes.WorkloadResource, delta bool, incr bool) (*plugintypes.SetNodeResourceUsageResponse, error)

	// GetMostIdleNode returns the most idle node for building
	GetMostIdleNode(ctx context.Context, nodenames []string) (*plugintypes.GetMostIdleNodeResponse, error)

	// FixNodeResource fixes the node resource usage by its workloads
	FixNodeResource(ctx context.Context, nodename string, workloadsResource []*plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error)

	// GetMetricsDescription returns metrics description
	GetMetricsDescription(ctx context.Context) (*plugintypes.GetMetricsDescriptionResponse, error)

	// GetMetrics resolves node resource info to metrics
	GetMetrics(ctx context.Context, podname, nodename string) (*plugintypes.GetMetricsResponse, error)

	// Name returns the name of plugin
	Name() string
}
