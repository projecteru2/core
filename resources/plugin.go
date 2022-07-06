package resources

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// Incr increase
	Incr = true

	// Decr decrease
	Decr = false

	getNodesCapacityCommand                 = "get-capacity"
	getNodeResourceInfoCommand              = "get-node"
	setNodeResourceInfoCommand              = "set-node"
	setNodeResourceUsageCommand             = "set-node-usage"
	setNodeResourceCapacityCommand          = "set-node-capacity"
	getDeployArgsCommand                    = "get-deploy-args"
	getReallocArgsCommand                   = "get-realloc-args"
	getRemapArgsCommand                     = "get-remap-args"
	addNodeCommand                          = "add-node"
	removeNodeCommand                       = "remove-node"
	getMostIdleNodeCommand                  = "get-idle"
	getMetricsDescriptionCommand            = "desc-metrics"
	resolveNodeResourceInfoToMetricsCommand = "resolve-metrics"
)

// Plugin indicate plugin methods
type Plugin interface {
	// GetDeployArgs tries to allocate resource, returns engine args for each workload, format: [{"cpus": 1.2}, {"cpus": 1.2}]
	// also returns resource args for each workload, format: [{"cpus": 1.2}, {"cpus": 1.2}]
	// pure calculation
	GetDeployArgs(ctx context.Context, nodeName string, deployCount int, resourceOpts coretypes.WorkloadResourceOpts) (*GetDeployArgsResponse, error)

	// GetReallocArgs tries to reallocate resource, returns engine args, delta resource args and final resource args.
	// should return error if resource of some node is not enough for the realloc operation.
	// pure calculation
	GetReallocArgs(ctx context.Context, nodeName string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) (*GetReallocArgsResponse, error)

	// GetRemapArgs tries to remap resources based on workload metadata and node resource usage, then returns engine args for workloads.
	// pure calculation
	GetRemapArgs(ctx context.Context, nodeName string, workloadMap map[string]*coretypes.Workload) (*GetRemapArgsResponse, error)

	// GetNodesDeployCapacity returns available nodes and total capacity
	GetNodesDeployCapacity(ctx context.Context, nodeNames []string, resourceOpts coretypes.WorkloadResourceOpts) (*GetNodesDeployCapacityResponse, error)

	// GetMostIdleNode returns the most idle node for building
	GetMostIdleNode(ctx context.Context, nodeNames []string) (*GetMostIdleNodeResponse, error)

	// GetNodeResourceInfo returns total resource info and available resource info of the node, format: {"cpu": 2}
	// also returns diffs, format: ["node.VolumeUsed != sum(workload.VolumeRequest"]
	GetNodeResourceInfo(ctx context.Context, nodeName string, workloads []*coretypes.Workload) (*GetNodeResourceInfoResponse, error)

	// FixNodeResource fixes the node resource usage by its workloads
	FixNodeResource(ctx context.Context, nodeName string, workloads []*coretypes.Workload) (*GetNodeResourceInfoResponse, error)

	// SetNodeResourceUsage sets the amount of allocated resource info
	SetNodeResourceUsage(ctx context.Context, nodeName string, nodeResourceOpts coretypes.NodeResourceOpts, nodeResourceArgs coretypes.NodeResourceArgs, workloadResourceArgs []coretypes.WorkloadResourceArgs, delta bool, incr bool) (*SetNodeResourceUsageResponse, error)

	// SetNodeResourceCapacity sets the amount of total resource info
	SetNodeResourceCapacity(ctx context.Context, nodeName string, nodeResourceOpts coretypes.NodeResourceOpts, nodeResourceArgs coretypes.NodeResourceArgs, delta bool, incr bool) (*SetNodeResourceCapacityResponse, error)

	// SetNodeResourceInfo sets both total node resource info and allocated resource info
	// used for rollback of RemoveNode
	// notice: here uses absolute values, not delta values
	SetNodeResourceInfo(ctx context.Context, nodeName string, resourceCapacity coretypes.NodeResourceArgs, resourceUsage coretypes.NodeResourceArgs) (*SetNodeResourceInfoResponse, error)

	// AddNode adds a node with requested resource, returns resource capacity and (empty) resource usage
	// should return error if the node already exists
	AddNode(ctx context.Context, nodeName string, resourceOpts coretypes.NodeResourceOpts, nodeInfo *enginetypes.Info) (*AddNodeResponse, error)

	// RemoveNode removes node
	RemoveNode(ctx context.Context, nodeName string) (*RemoveNodeResponse, error)

	// GetMetricsDescription returns metrics description
	GetMetricsDescription(ctx context.Context) (*GetMetricsDescriptionResponse, error)

	// ConvertNodeResourceInfoToMetrics resolves node resource info to metrics
	ConvertNodeResourceInfoToMetrics(ctx context.Context, podName string, nodeName string, nodeResourceInfo *NodeResourceInfo) (*ConvertNodeResourceInfoToMetricsResponse, error)

	// Name returns the name of plugin
	Name() string
}

// Manager indicate manages
type Manager interface {
	// GetMostIdleNode .
	GetMostIdleNode(context.Context, []string) (string, error)

	GetNodesDeployCapacity(context.Context, []string, types.WorkloadResourceOpts) (map[string]*NodeCapacityInfo, int, error)

	SetNodeResourceCapacity(context.Context, string, types.NodeResourceOpts, map[string]types.NodeResourceArgs, bool, bool) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, error)

	GetNodeResourceInfo(context.Context, string, []*types.Workload, bool, []string) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, []string, error)

	SetNodeResourceUsage(context.Context, string, types.NodeResourceOpts, map[string]types.NodeResourceArgs, []map[string]types.WorkloadResourceArgs, bool, bool) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, error)

	Alloc(context.Context, string, int, types.WorkloadResourceOpts) ([]types.EngineArgs, []map[string]types.WorkloadResourceArgs, error)
	RollbackAlloc(context.Context, string, []map[string]types.WorkloadResourceArgs) error
	Realloc(context.Context, string, map[string]types.WorkloadResourceArgs, types.WorkloadResourceOpts) (types.EngineArgs, map[string]types.WorkloadResourceArgs, map[string]types.WorkloadResourceArgs, error)
	RollbackRealloc(context.Context, string, map[string]types.WorkloadResourceArgs) error

	GetMetricsDescription(context.Context) ([]*MetricsDescription, error)
	ConvertNodeResourceInfoToMetrics(context.Context, string, string, map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs) ([]*Metrics, error)

	AddNode(context.Context, string, types.NodeResourceOpts, *enginetypes.Info) (map[string]types.NodeResourceArgs, map[string]types.NodeResourceArgs, error)
	RemoveNode(context.Context, string) error

	GetRemapArgs(context.Context, string, map[string]*types.Workload) (map[string]types.EngineArgs, error)

	// GetPlugins for testing
	GetPlugins() []Plugin
}
