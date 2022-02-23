package mocks

import (
	"context"

	"github.com/sanity-io/litter"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/resources"
	coretypes "github.com/projecteru2/core/types"
)

func NewMockCpuPlugin() *Plugin {
	m := &Plugin{}
	m.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodesDeployCapacityResponse{
		Nodes: map[string]*resources.NodeCapacityInfo{
			"node1": {
				NodeName: "node1",
				Capacity: 1,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   1,
			},
			"node2": {
				NodeName: "node2",
				Capacity: 2,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   1,
			},
		},
		Total: 6,
	}, nil)

	m.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, deployCount int, rawRequest coretypes.WorkloadResourceOpts) *resources.GetDeployArgsResponse {
		log.Infof(ctx, "[Alloc] alloc, node %s, deploy count %v, request %+v", node, deployCount, rawRequest)
		return &resources.GetDeployArgsResponse{
			EngineArgs: []coretypes.EngineArgs{
				map[string]interface{}{
					"cpu":  1.2,
					"file": []string{"cpu"},
				},
			},
			ResourceArgs: []coretypes.WorkloadResourceArgs{
				map[string]interface{}{
					"cpu":  1.2,
					"file": []string{"cpu"},
				},
			},
		}
	}, nil)

	m.On("UpdateNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, func(ctx context.Context, node string, resourceArgs []coretypes.WorkloadResourceArgs, direction bool) error {
		log.Infof(ctx, "[UpdateNodeResourceUsage] cpu-plugin UpdateNodeResourceUsage, incr %v, node %s, resource args %+v", direction, node, litter.Sdump(resourceArgs))
		return nil
	})

	m.On("Name").Return("cpu-plugin")

	m.On("GetRemapArgs", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, workloadMap map[string]*coretypes.Workload) *resources.GetRemapArgsResponse {
		log.Infof(ctx, "[GetRemapArgs] node %v", node)
		res := map[string]coretypes.EngineArgs{}
		for workloadID := range workloadMap {
			res[workloadID] = map[string]interface{}{
				"cpuset-cpus": []string{"0-65535"}, // I'm rich!
			}
		}
		return &resources.GetRemapArgsResponse{EngineArgsMap: res}
	}, nil)

	m.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&resources.GetNodeResourceInfoResponse{
			ResourceInfo: &resources.NodeResourceInfo{
				Capacity: coretypes.NodeResourceArgs{
					"cpu": 100,
				},
				Usage: coretypes.NodeResourceArgs{
					"cpu": 100,
				},
			},
			Diffs: []string{"cpu is sleepy"},
		},
		nil,
	)

	m.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) *resources.GetReallocArgsResponse {
		log.Infof(ctx, "[Realloc] cpu-plugin realloc workloads, resource opts: %v, resource args: %v", resourceOpts, originResourceArgs)
		return &resources.GetReallocArgsResponse{
			EngineArgs:   coretypes.EngineArgs{"cpu": 10086},
			Delta:        coretypes.WorkloadResourceArgs{"cpu": 10086},
			ResourceArgs: coretypes.WorkloadResourceArgs{"cpu": 10086},
		}
	}, nil)

	m.On("AddNode", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, rawRequest coretypes.NodeResourceOpts) *resources.AddNodeResponse {
		log.Infof(ctx, "cpu-plugin add node %v, req: %+v", node, rawRequest)
		return &resources.AddNodeResponse{
			Capacity: coretypes.NodeResourceArgs{
				"cpu": 65535,
			},
			Usage: coretypes.NodeResourceArgs{
				"cpu": 0,
			},
		}
	}, nil)

	m.On("RemoveNode", mock.Anything, mock.Anything).Return(nil, func(ctx context.Context, node string) error {
		log.Infof(ctx, "cpu-plugin remove node %v", node)
		return nil
	}, nil)

	return m
}

func NewMockMemPlugin() *Plugin {
	m := &Plugin{}
	m.On("LockNodes", mock.Anything, mock.Anything).Return(nil)
	m.On("UnlockNodes", mock.Anything, mock.Anything).Return(nil)
	m.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetNodesDeployCapacityResponse{
		Nodes: map[string]*resources.NodeCapacityInfo{
			"node1": {
				NodeName: "node1",
				Capacity: 1,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   1,
			},
			"node2": {
				NodeName: "node2",
				Capacity: 2,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   1,
			},
			"node3": {
				NodeName: "node3",
				Capacity: 3,
				Usage:    0.5,
				Rate:     0.5,
				Weight:   1,
			},
		},
		Total: 6,
	}, nil)

	m.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, deployCount int, rawRequest coretypes.WorkloadResourceOpts) *resources.GetDeployArgsResponse {
		log.Infof(ctx, "[Alloc] node %v, deploy count %v, raw request %v", node, deployCount, litter.Sdump(rawRequest))
		return &resources.GetDeployArgsResponse{
			EngineArgs: []coretypes.EngineArgs{
				map[string]interface{}{
					"mem":  "1PB",
					"file": []string{"mem"},
				},
			},
			ResourceArgs: []coretypes.WorkloadResourceArgs{
				map[string]interface{}{
					"mem":  "1PB",
					"file": []string{"mem"},
				},
			},
		}
	}, nil)

	m.On("UpdateNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, func(ctx context.Context, node string, resourceArgs []coretypes.WorkloadResourceArgs, direction bool) error {
		log.Infof(ctx, "[UpdateNodeResourceUsage] mem-plugin UpdateNodeResourceUsage, incr %v, node %s, resource args %+v", direction, node, litter.Sdump(resourceArgs))
		return nil
	})

	m.On("Name").Return("mem-plugin")

	m.On("GetRemapArgs", mock.Anything, mock.Anything, mock.Anything).Return(&resources.GetRemapArgsResponse{}, nil)

	m.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		&resources.GetNodeResourceInfoResponse{
			ResourceInfo: &resources.NodeResourceInfo{
				Capacity: coretypes.NodeResourceArgs{
					"mem_cap": "10000PB",
				},
				Usage: coretypes.NodeResourceArgs{
					"mem_cap": "10000PB",
				},
			},
			Diffs: []string{"the mem_cap doesn't look like a machine on earth"},
		}, nil)

	m.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, originResourceArgs coretypes.WorkloadResourceArgs, resourceOpts coretypes.WorkloadResourceOpts) *resources.GetReallocArgsResponse {
		log.Infof(ctx, "[Realloc] mem-plugin realloc workloads, resource opts: %v, resource args: %v", resourceOpts, originResourceArgs)
		return &resources.GetReallocArgsResponse{
			EngineArgs: coretypes.EngineArgs{
				"mem": "1000000PB",
			},
			Delta: coretypes.WorkloadResourceArgs{
				"mem": "1000000PB",
			},
			ResourceArgs: coretypes.WorkloadResourceArgs{
				"mem": "1000000PB",
			},
		}
	}, nil)

	m.On("AddNode", mock.Anything, mock.Anything, mock.Anything).Return(func(ctx context.Context, node string, rawRequest coretypes.NodeResourceOpts) *resources.AddNodeResponse {
		log.Infof(ctx, "mem-plugin add node %v, req: %+v", node, rawRequest)
		return &resources.AddNodeResponse{
			Capacity: coretypes.NodeResourceArgs{
				"mem": 65535,
			},
			Usage: coretypes.NodeResourceArgs{
				"mem": 65535,
			},
		}
	}, nil)

	m.On("RemoveNode", mock.Anything, mock.Anything).Return(func(ctx context.Context, node string) *resources.RemoveNodeResponse {
		log.Infof(ctx, "mem-plugin remove node %v", node)
		return nil
	}, nil)

	return m
}
