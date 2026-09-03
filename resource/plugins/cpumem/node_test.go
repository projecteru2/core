package cpumem

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/resource/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestAddNode(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]
	nodeForAdd := "test2"

	req := plugintypes.NodeResourceRequest{
		"numa-cpu": []string{"0", "1"},
	}

	info := &enginetypes.Info{NCPU: 2, MemTotal: 4 * units.GB}

	_, err := cm.AddNode(ctx, node, req, info)
	assert.Equal(t, err, coretypes.ErrNodeExists)

	r, err := cm.AddNode(ctx, nodeForAdd, req, info)
	assert.Nil(t, err)
	assert.Equal(t, r.Capacity["memory"], float64(4*units.GB*rate/10))
}

func TestAddNodeKeepsTheRequestedNUMATopology(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	req := plugintypes.NodeResourceRequest{
		"numa-cpu":    []string{"0", "1"},
		"numa-memory": []string{"1073741824", "1073741824"},
	}

	_, err := cm.AddNode(ctx, "numa-node", req, &enginetypes.Info{NCPU: 2, MemTotal: 4 * units.GB})
	assert.Nil(t, err)

	stored, err := cm.doGetNodeResourceInfo(ctx, "numa-node")
	assert.Nil(t, err)
	assert.Equal(t, types.NUMA{"0": "0", "1": "1"}, stored.Capacity.NUMA)
	assert.Equal(t, types.NUMAMemory{"0": 1073741824, "1": 1073741824}, stored.Capacity.NUMAMemory)
}

func TestRemoveNode(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]
	nodeForDel := "test2"

	_, err := cm.RemoveNode(ctx, node)
	assert.Nil(t, err)
	_, err = cm.RemoveNode(ctx, nodeForDel)
	assert.Nil(t, err)
}

func TestGetNodesDeployCapacityWithCPUBind(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 4*units.GB, 100, 0)

	req := plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    0.5,
		"memory-request": "1",
	}

	_, err := cm.GetNodesDeployCapacity(ctx, []string{"xxx"}, req)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total >= 1)

	req = plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    2,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total < 3)

	req = plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    3,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total < 2)

	req = plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total < 5)

	nodes = generateNodes(ctx, t, cm, 1, 4, 12*units.GB, 100, 10)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 14, 12*units.GB, 100, 11)...)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 12, 12*units.GB, 100, 12)...)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 18, 12*units.GB, 100, 13)...)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 8, 12*units.GB, 100, 14)...)

	req = plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1.7,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 28)
}

func TestGetNodesDeployCapacityWithMemoryAndCPUBind(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 1024, 100, 0)

	req := plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    0.1,
		"memory-request": "1024",
	}

	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 2)

	req["memory-request"] = "1025"
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 0)
}

func TestGetNodesDeployCapacityWithMaxShareLimit(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	cm.config.Scheduler.MaxShare = 2
	nodes := generateNodes(ctx, t, cm, 1, 6, 12*units.GB, 100, 0)
	node := nodes[0]

	req := plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1.7,
		"memory-request": "1",
	}

	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 2)

	resource := plugintypes.NodeResource{
		"cpu": 4.0,
		"cpu_map": map[string]int64{
			"0": 0,
			"1": 0,
			"2": 100,
			"3": 100,
		},
		"memory": 12 * units.GB,
	}

	_, err = cm.SetNodeResourceCapacity(ctx, node, resource, nil, false, true)
	assert.Nil(t, err)

	req["cpu-request"] = 1.2
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 1)
}

func TestGetNodesDeployCapacityWithMemory(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 4*units.GiB, 100, 0)

	req := plugintypes.WorkloadResourceRequest{
		"memory-request": "-1",
	}

	_, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.True(t, errors.Is(err, types.ErrInvalidMemory))

	req = plugintypes.WorkloadResourceRequest{
		"cpu-request":    1,
		"memory-request": fmt.Sprintf("%v", 512*units.MB),
	}

	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 16)

	req = plugintypes.WorkloadResourceRequest{
		"memory-request": fmt.Sprintf("%v", 512*units.MB),
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 16)

	req = plugintypes.WorkloadResourceRequest{
		"cpu-request":    3,
		"memory-request": fmt.Sprintf("%v", 512*units.MB),
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 0)

	req = plugintypes.WorkloadResourceRequest{
		"cpu-request": 1,
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, math.MaxInt)
}

func TestSetNodeResourceCapacity(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 2*units.GB, 100, 0)
	node := nodes[0]

	_, err := cm.GetNodeResourceInfo(ctx, node, nil)
	assert.Nil(t, err)

	nodeResource := plugintypes.NodeResource{
		"cpu_map": map[string]int{
			"2": 100,
			"3": 100,
		},
		"numa_memory": types.NUMAMemory{
			"0": units.GiB,
			"1": units.GiB,
		},
		"numa": types.NUMA{
			"0": "0",
			"1": "0",
			"2": "1",
			"3": "1",
		},
	}

	newNodeResource := plugintypes.NodeResource{
		"cpu_map": map[string]int{
			"0": 100,
			"1": 100,
		},
		"memory": 2 * units.GB,
		"xxxx":   map[string]any{"cpu": ""},
	}

	gb := fmt.Sprintf("%v", units.GB)
	nodeResourceRequest := plugintypes.NodeResourceRequest{
		"cpu":         "2:100,3:100",
		"numa-memory": []string{gb, gb},
		"numa-cpu":    []string{"0,1", "2,3"},
	}

	noChangeRequest := plugintypes.NodeResourceRequest{
		"cpu":    "0:100,1:100,2:100,3:100",
		"memory": fmt.Sprintf("%v", 2*units.GB),
	}

	r, err := cm.SetNodeResourceCapacity(ctx, node, nodeResource, nil, true, true)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 4)

	r, err = cm.SetNodeResourceCapacity(ctx, node, nodeResource, nil, true, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Len(t, r.After["numa_memory"], 2)
	assert.Len(t, r.After["numa"], 4)

	r, err = cm.SetNodeResourceCapacity(ctx, node, nil, nodeResourceRequest, true, true)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 4)
	assert.Len(t, r.After["numa_memory"], 2)
	assert.Len(t, r.After["numa"], 4)

	r, err = cm.SetNodeResourceCapacity(ctx, node, nil, nodeResourceRequest, true, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Len(t, r.After["numa_memory"], 2)
	assert.Len(t, r.After["numa"], 4)

	r, err = cm.SetNodeResourceCapacity(ctx, node, nil, noChangeRequest, false, true)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 4)

	r, err = cm.SetNodeResourceCapacity(ctx, node, newNodeResource, nil, false, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Len(t, r.After["numa"], 0)
}

func TestGetAndFixNodeResourceInfo(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	_, err := cm.GetNodeResourceInfo(ctx, "xxx", nil)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	r, err := cm.GetNodeResourceInfo(ctx, node, nil)
	assert.Nil(t, err)
	assert.Len(t, r.Diffs, 0)

	r.Capacity["numa"] = types.NUMA{"0": "0", "1": "1"}
	r.Capacity["numa_memory"] = types.NUMAMemory{"0": units.GB, "1": units.GB}

	_, err = cm.SetNodeResourceInfo(ctx, node, r.Capacity, r.Usage)
	assert.Nil(t, err)

	workloadsResource := []plugintypes.WorkloadResource{
		{
			"cpu_request":    2.0,
			"cpu_map":        types.CPUMap{"0": 100, "1": 100},
			"memory_request": 2 * units.GB,
			"numa_memory":    types.NUMAMemory{"0": units.GB, "1": units.GB},
		},
	}
	r, err = cm.GetNodeResourceInfo(ctx, node, workloadsResource)
	assert.Nil(t, err)
	assert.Len(t, r.Diffs, 6)

	r, err = cm.FixNodeResource(ctx, node, workloadsResource)
	assert.Nil(t, err)
	assert.Len(t, r.Diffs, 6)
	assert.Len(t, r.Usage["numa_memory"], 2)
}

func TestFixNodeResourceReturnsTheWriteError(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	node := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)[0]
	cm.store = &putErrorStore{Store: cm.store}

	_, err := cm.FixNodeResource(ctx, node, []plugintypes.WorkloadResource{{"cpu_request": 1.0, "memory_request": units.GB}})
	assert.ErrorIs(t, err, coretypes.ErrMockError)
}

func TestSetNodeResourceInfo(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	r, err := cm.GetNodeResourceInfo(ctx, node, nil)
	assert.Nil(t, err)

	_, err = cm.SetNodeResourceInfo(ctx, "node-2", r.Capacity, r.Usage)
	assert.Nil(t, err)
}

func TestSetNodeResourceUsage(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	_, err := cm.GetNodeResourceInfo(ctx, node, nil)
	assert.Nil(t, err)

	nodeResource := plugintypes.NodeResource{
		"cpu_map": map[string]int{
			"0": 100,
			"1": 100,
		},
		"memory": 2 * units.GB,
	}

	nodeResourceRequest := plugintypes.NodeResourceRequest{
		"cpu":    "0:100,1:100",
		"memory": fmt.Sprintf("%v", 2*units.GB),
	}

	workloadsResource := []plugintypes.WorkloadResource{
		{
			"cpu_request": 2.0,
			"cpu_map": types.CPUMap{
				"0": 100,
				"1": 100,
			},
			"memory_request": 2 * units.GB,
		},
	}

	r, err := cm.SetNodeResourceUsage(ctx, node, nodeResource, nil, nil, true, true)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)

	r, err = cm.SetNodeResourceUsage(ctx, node, nodeResource, nil, nil, true, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Equal(t, r.After["cpu"], 0.0)
	assert.Equal(t, r.After["memory"], 0.0)

	r, err = cm.SetNodeResourceUsage(ctx, node, nil, nodeResourceRequest, nil, true, true)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)

	r, err = cm.SetNodeResourceUsage(ctx, node, nil, nodeResourceRequest, nil, true, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Equal(t, r.After["cpu"], 0.0)
	assert.Equal(t, r.After["memory"], 0.0)

	r, err = cm.SetNodeResourceUsage(ctx, node, nil, nil, workloadsResource, true, true)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)

	r, err = cm.SetNodeResourceUsage(ctx, node, nil, nil, workloadsResource, true, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Equal(t, r.After["cpu"], 0.0)
	assert.Equal(t, r.After["memory"], 0.0)

	r, err = cm.SetNodeResourceUsage(ctx, node, nil, nil, nil, true, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Equal(t, r.After["cpu"], 0.0)
	assert.Equal(t, r.After["memory"], 0.0)

	r, err = cm.SetNodeResourceUsage(ctx, node, nil, nodeResourceRequest, nil, false, false)
	assert.Nil(t, err)
	assert.Len(t, r.After["cpu_map"], 2)
	assert.Equal(t, r.After["cpu"], 2.0)
	assert.Equal(t, r.After["memory"], float64(2*units.GB))
}

func TestGetMostIdleNode(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 2*units.GB, 100, 0)
	usage := plugintypes.NodeResourceRequest{"memory": "100"}

	_, err := cm.SetNodeResourceUsage(ctx, nodes[1], nil, usage, nil, false, false)
	assert.Nil(t, err)

	r, err := cm.GetMostIdleNode(ctx, nodes)
	assert.Nil(t, err)
	assert.Equal(t, r.Nodename, nodes[0])

	nodes = append(nodes, "node-x")
	_, err = cm.GetMostIdleNode(ctx, nodes)
	assert.Error(t, err)
}

func BenchmarkGetNodesCapacity(b *testing.B) {
	ctx := b.Context()
	cm := initCPUMEM(b)
	nodes := generateNodes(ctx, b, cm, 1000, 24, 128*units.GB, 100, 0)
	req := plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1.3,
		"memory-request": "1",
	}

	for b.Loop() {
		_, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
		assert.Nil(b, err)
	}
}

func TestAddNodeSplitsMemoryPerNUMANode(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)

	req := plugintypes.NodeResourceRequest{
		"cpu":      8,
		"memory":   8 * units.GB,
		"numa-cpu": []string{"0,1,2,3", "4,5,6,7"},
	}

	r, err := cm.AddNode(ctx, "numa-node", req, nil)
	assert.Nil(t, err)

	numaMemory, ok := r.Capacity["numa_memory"].(map[string]any)
	assert.True(t, ok)
	assert.Equal(t, map[string]any{"0": float64(4 * units.GB), "1": float64(4 * units.GB)}, numaMemory)
}

func BenchmarkGetNodesCapacityScaling(b *testing.B) {
	for _, tc := range []struct{ nodes, cores int }{{100, 24}, {1000, 24}, {100, 64}, {100, 128}, {100, 256}, {1000, 128}} {
		b.Run(fmt.Sprintf("nodes=%d/cores=%d", tc.nodes, tc.cores), func(b *testing.B) {
			ctx := b.Context()
			cm := initCPUMEM(b)
			nodes := generateNodes(ctx, b, cm, tc.nodes, tc.cores, 128*units.GB, 100, 0)
			req := plugintypes.WorkloadResourceRequest{"cpu-bind": true, "cpu-request": 1.3, "memory-request": "1"}
			b.ResetTimer()
			for b.Loop() {
				if _, err := cm.GetNodesDeployCapacity(ctx, nodes, req); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

type putErrorStore struct {
	Store
}

func (putErrorStore) Put(context.Context, map[string]string) error {
	return coretypes.ErrMockError
}
