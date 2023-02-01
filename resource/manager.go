package resource

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	"github.com/projecteru2/core/types"
)

// Manager indicate manages
// coretypes --> manager to rawparams --> plugins types
type Manager interface {
	AddNode(context.Context, string, resourcetypes.Resources, *enginetypes.Info) (resourcetypes.Resources, error)
	RemoveNode(context.Context, string) error
	GetNodesDeployCapacity(context.Context, []string, resourcetypes.Resources) (map[string]*plugintypes.NodeDeployCapacity, int, error)
	SetNodeResourceCapacity(context.Context, string, resourcetypes.Resources, resourcetypes.Resources, bool, bool) (resourcetypes.Resources, resourcetypes.Resources, error)
	SetNodeResourceUsage(context.Context, string, resourcetypes.Resources, resourcetypes.Resources, []resourcetypes.Resources, bool, bool) (resourcetypes.Resources, resourcetypes.Resources, error)
	GetNodeResourceInfo(context.Context, string, []*types.Workload, bool) (resourcetypes.Resources, resourcetypes.Resources, []string, error)
	GetMostIdleNode(context.Context, []string) (string, error)

	Alloc(context.Context, string, int, resourcetypes.Resources) ([]resourcetypes.Resources, []resourcetypes.Resources, error)
	RollbackAlloc(context.Context, string, []resourcetypes.Resources) error
	Realloc(context.Context, string, resourcetypes.Resources, resourcetypes.Resources) (resourcetypes.Resources, resourcetypes.Resources, resourcetypes.Resources, error)
	RollbackRealloc(context.Context, string, resourcetypes.Resources) error
	Remap(context.Context, string, []*types.Workload) (resourcetypes.Resources, error)

	GetNodeMetrics(context.Context, *types.Node) ([]*plugintypes.Metrics, error)
	GetMetricsDescription(context.Context) ([]*plugintypes.MetricsDescription, error)
}
