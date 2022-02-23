package resources

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	coretypes "github.com/projecteru2/core/types"
)

const (
	// Incr increase
	Incr = true

	// Decr decrease
	Decr = false

	getNodesCapacityCommand        = "get-capacity"
	getNodeResourceInfoCommand     = "get-node"
	setNodeResourceInfoCommand     = "set-node"
	setNodeResourceUsageCommand    = "set-node-usage"
	setNodeResourceCapacityCommand = "set-node-capacity"
	getDeployArgsCommand           = "get-deploy-args"
	getReallocArgsCommand          = "get-realloc-args"
	getRemapArgsCommand            = "get-remap-args"
	addNodeCommand                 = "add-node"
	removeNodeCommand              = "remove-node"
	getMostIdleNodeCommand         = "get-idle"
)

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

	// Name returns the name of plugin
	Name() string
}
