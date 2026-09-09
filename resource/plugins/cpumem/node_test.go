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

func TestGetNodesResourceInfo(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 4*units.GB, 100, 0)

	resp, err := cm.GetNodesResourceInfo(ctx, append(nodes, "never-added"))
	assert.NoError(t, err)
	assert.Len(t, resp.NodeResourceInfoMap, 2)
	for _, node := range nodes {
		assert.EqualValues(t, 2, resp.NodeResourceInfoMap[node].Capacity["cpu"])
		assert.EqualValues(t, 0, resp.NodeResourceInfoMap[node].Usage["cpu"])
	}
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

	tests := []struct {
		name       string
		cpuRequest any
		check      func(t *testing.T, total int)
	}{
		{"half core request", 0.5, func(t *testing.T, total int) { assert.True(t, total >= 1) }},
		{"two core request", 2, func(t *testing.T, total int) { assert.True(t, total < 3) }},
		{"three core request", 3, func(t *testing.T, total int) { assert.True(t, total < 2) }},
		{"one core request", 1, func(t *testing.T, total int) { assert.True(t, total < 5) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := plugintypes.WorkloadResourceRequest{
				"cpu-bind":       true,
				"cpu-request":    tt.cpuRequest,
				"memory-request": "1",
			}
			r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
			assert.Nil(t, err)
			tt.check(t, r.Total)
		})
	}

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
	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
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

	tests := []struct {
		name string
		req  plugintypes.WorkloadResourceRequest
		want int
	}{
		{"cpu and memory request", plugintypes.WorkloadResourceRequest{"cpu-request": 1, "memory-request": fmt.Sprintf("%v", 512*units.MB)}, 16},
		{"memory request only", plugintypes.WorkloadResourceRequest{"memory-request": fmt.Sprintf("%v", 512*units.MB)}, 16},
		{"cpu request exceeds capacity", plugintypes.WorkloadResourceRequest{"cpu-request": 3, "memory-request": fmt.Sprintf("%v", 512*units.MB)}, 0},
		{"cpu request only", plugintypes.WorkloadResourceRequest{"cpu-request": 1}, math.MaxInt},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := cm.GetNodesDeployCapacity(ctx, nodes, tt.req)
			assert.Nil(t, err)
			assert.Equal(t, r.Total, tt.want)
		})
	}
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

	tests := []struct {
		name     string
		resource plugintypes.NodeResource
		request  plugintypes.NodeResourceRequest
		delta    bool
		incr     bool
		check    func(t *testing.T, after plugintypes.NodeResource)
	}{
		{
			name:     "resource delta incr",
			resource: nodeResource,
			delta:    true,
			incr:     true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 4)
			},
		},
		{
			name:     "resource delta decr",
			resource: nodeResource,
			delta:    true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Len(t, after["numa_memory"], 2)
				assert.Len(t, after["numa"], 4)
			},
		},
		{
			name:    "request delta incr",
			request: nodeResourceRequest,
			delta:   true,
			incr:    true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 4)
				assert.Len(t, after["numa_memory"], 2)
				assert.Len(t, after["numa"], 4)
			},
		},
		{
			name:    "request delta decr",
			request: nodeResourceRequest,
			delta:   true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Len(t, after["numa_memory"], 2)
				assert.Len(t, after["numa"], 4)
			},
		},
		{
			name:    "request no delta incr",
			request: noChangeRequest,
			incr:    true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 4)
			},
		},
		{
			name:     "resource no delta decr",
			resource: newNodeResource,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Len(t, after["numa"], 0)
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := cm.SetNodeResourceCapacity(ctx, node, tt.resource, tt.request, tt.delta, tt.incr)
			assert.Nil(t, err)
			tt.check(t, r.After)
		})
	}
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

	tests := []struct {
		name              string
		resource          plugintypes.NodeResource
		resourceRequest   plugintypes.NodeResourceRequest
		workloadsResource []plugintypes.WorkloadResource
		delta             bool
		incr              bool
		check             func(t *testing.T, after plugintypes.NodeResource)
	}{
		{
			name:     "resource delta incr",
			resource: nodeResource,
			delta:    true,
			incr:     true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
			},
		},
		{
			name:     "resource delta decr",
			resource: nodeResource,
			delta:    true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Equal(t, after["cpu"], 0.0)
				assert.Equal(t, after["memory"], 0.0)
			},
		},
		{
			name:            "request delta incr",
			resourceRequest: nodeResourceRequest,
			delta:           true,
			incr:            true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
			},
		},
		{
			name:            "request delta decr",
			resourceRequest: nodeResourceRequest,
			delta:           true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Equal(t, after["cpu"], 0.0)
				assert.Equal(t, after["memory"], 0.0)
			},
		},
		{
			name:              "workloads delta incr",
			workloadsResource: workloadsResource,
			delta:             true,
			incr:              true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
			},
		},
		{
			name:              "workloads delta decr",
			workloadsResource: workloadsResource,
			delta:             true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Equal(t, after["cpu"], 0.0)
				assert.Equal(t, after["memory"], 0.0)
			},
		},
		{
			name:  "no input delta decr",
			delta: true,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Equal(t, after["cpu"], 0.0)
				assert.Equal(t, after["memory"], 0.0)
			},
		},
		{
			name:            "request no delta",
			resourceRequest: nodeResourceRequest,
			check: func(t *testing.T, after plugintypes.NodeResource) {
				assert.Len(t, after["cpu_map"], 2)
				assert.Equal(t, after["cpu"], 2.0)
				assert.Equal(t, after["memory"], float64(2*units.GB))
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, err := cm.SetNodeResourceUsage(ctx, node, tt.resource, tt.resourceRequest, tt.workloadsResource, tt.delta, tt.incr)
			assert.Nil(t, err)
			tt.check(t, r.After)
		})
	}
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
