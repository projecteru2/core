package plugins

import (
	"context"

	"github.com/cockroachdb/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
)

const (
	Incr = true
	Decr = false
)

// ErrVerbNotSupported marks a verb the plugin did not advertise; cobalt leaves the plugin out of that call.
var ErrVerbNotSupported = errors.New("verb not supported by the plugin")

type Plugin interface {
	// CalculateDeploy plans per-workload engine and resource params without writing anything.
	CalculateDeploy(ctx context.Context, nodename string, deployCount int, resourceRequest plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateDeployResponse, error)

	// CalculateRealloc plans a workload's new engine params, delta and final resource without writing anything.
	CalculateRealloc(ctx context.Context, nodename string, resource plugintypes.WorkloadResource, resourceRequest plugintypes.WorkloadResourceRequest) (*plugintypes.CalculateReallocResponse, error)

	// CalculateRemap plans engine params per workload ID from the node's usage without writing anything.
	CalculateRemap(ctx context.Context, nodename string, workloadsResource map[string]plugintypes.WorkloadResource) (*plugintypes.CalculateRemapResponse, error)

	// AddNode records a node's capacity and an empty usage, and fails on a node it already has.
	AddNode(ctx context.Context, nodename string, resource plugintypes.NodeResourceRequest, info *enginetypes.Info) (*plugintypes.AddNodeResponse, error)

	RemoveNode(ctx context.Context, nodename string) (*plugintypes.RemoveNodeResponse, error)

	GetNodesDeployCapacity(ctx context.Context, nodenames []string, resource plugintypes.WorkloadResourceRequest) (*plugintypes.GetNodesDeployCapacityResponse, error)

	SetNodeResourceCapacity(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, delta, incr bool) (*plugintypes.SetNodeResourceCapacityResponse, error)

	// GetNodeResourceInfo returns total resource info and available resource info of the node, format: {"cpu": 2}
	GetNodeResourceInfo(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error)

	// SetNodeResourceInfo stores a node's capacity and usage as absolute values.
	SetNodeResourceInfo(ctx context.Context, nodename string, capacity, usage plugintypes.NodeResource) (*plugintypes.SetNodeResourceInfoResponse, error)

	SetNodeResourceUsage(ctx context.Context, nodename string, resource plugintypes.NodeResource, resourceRequest plugintypes.NodeResourceRequest, workloadsResource []plugintypes.WorkloadResource, delta, incr bool) (*plugintypes.SetNodeResourceUsageResponse, error)

	GetMostIdleNode(ctx context.Context, nodenames []string) (*plugintypes.GetMostIdleNodeResponse, error)

	FixNodeResource(ctx context.Context, nodename string, workloadsResource []plugintypes.WorkloadResource) (*plugintypes.GetNodeResourceInfoResponse, error)

	GetMetricsDescription(ctx context.Context) (*plugintypes.GetMetricsDescriptionResponse, error)

	GetMetrics(ctx context.Context, nodes []plugintypes.NodeRef) (*plugintypes.GetMetricsResponse, error)

	Name() string
}
