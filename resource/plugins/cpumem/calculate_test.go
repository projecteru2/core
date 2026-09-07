package cpumem

import (
	"fmt"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resource/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestCalculateDeploy(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	tests := []struct {
		name    string
		node    string
		count   int
		req     plugintypes.WorkloadResourceRequest
		wantErr error
	}{
		{"negative cpu", node, 100, plugintypes.WorkloadResourceRequest{"cpu-bind": true, "cpu-request": -1}, types.ErrInvalidCPU},
		{"unknown node", "xxx", 100, plugintypes.WorkloadResourceRequest{"cpu-bind": true, "cpu-request": 1}, coretypes.ErrInvaildCount},
		{"fractional bind fits", node, 1, plugintypes.WorkloadResourceRequest{"cpu-bind": true, "cpu-request": 1.1}, nil},
		{"bind over the cores", node, 1, plugintypes.WorkloadResourceRequest{"cpu-bind": true, "cpu-request": 2.2}, coretypes.ErrInsufficientCapacity},
		{"bind over the count", node, 3, plugintypes.WorkloadResourceRequest{"cpu-bind": true, "cpu-request": 1}, coretypes.ErrInsufficientCapacity},
		{"memory only", node, 1, plugintypes.WorkloadResourceRequest{"memory-request": fmt.Sprintf("%v", units.GB)}, nil},
		{"cpu over the node", node, 1, plugintypes.WorkloadResourceRequest{"memory-request": fmt.Sprintf("%v", units.GB), "cpu-request": 1000}, coretypes.ErrInsufficientCapacity},
		{"memory over the node", node, 1, plugintypes.WorkloadResourceRequest{"memory-request": fmt.Sprintf("%v", 5*units.GB), "cpu-request": 1}, coretypes.ErrInsufficientCapacity},
		{"zero memory", node, 1, plugintypes.WorkloadResourceRequest{"memory-request": "0", "cpu-request": 1}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cm.CalculateDeploy(ctx, tt.node, tt.count, tt.req)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}

	resource := plugintypes.NodeResource{
		"cpu": 4.0,
		"cpu_map": map[string]int64{
			"0": 100,
			"1": 100,
			"2": 100,
			"3": 100,
		},
		"memory": 4 * units.GB,
		"numa_memory": map[string]int64{
			"0": 2 * units.GB,
			"1": 2 * units.GB,
		},
		"numa": map[string]string{
			"0": "0",
			"2": "0",
			"1": "1",
			"3": "1",
		},
	}
	_, err := cm.SetNodeResourceCapacity(ctx, node, resource, nil, false, true)
	assert.NoError(t, err)

	req := plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"memory":      fmt.Sprintf("%v", units.GB),
		"cpu-request": 1.3,
	}
	r, err := cm.CalculateDeploy(ctx, node, 1, req)
	assert.NoError(t, err)
	assert.NotNil(t, r.WorkloadsResource)
}

func TestCalculateRealloc(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	resource := plugintypes.NodeResource{
		"cpu": 2.0,
		"cpu_map": map[string]int64{
			"0": 100,
			"1": 100,
		},
		"memory": 4 * units.GB,
		"numa_memory": map[string]int64{
			"0": units.GB,
			"1": units.GB,
		},
		"numa": map[string]string{
			"0": "0",
			"1": "1",
		},
	}
	_, err := cm.SetNodeResourceCapacity(ctx, node, resource, nil, false, true)
	assert.NoError(t, err)

	origin := plugintypes.WorkloadResource{
		"cpu_request":    1,
		"cpu_limit":      1,
		"memory_request": units.GB,
		"memory_limit":   units.GB,
		"cpu_map":        types.CPUMap{"0": 100},
		"numa_memory":    types.NUMAMemory{"0": units.GB},
		"numa_node":      "0",
	}
	tests := []struct {
		name    string
		node    string
		origin  plugintypes.WorkloadResource
		req     plugintypes.WorkloadResourceRequest
		wantErr error
	}{
		{"unknown node", "xxx", plugintypes.WorkloadResource{}, plugintypes.WorkloadResourceRequest{}, coretypes.ErrInvaildCount},
		{"cpu below zero", node, origin, plugintypes.WorkloadResourceRequest{"keep-cpu-bind": true, "cpu-request": -3}, types.ErrInvalidCPU},
		{"cpu over the node", node, origin, plugintypes.WorkloadResourceRequest{"keep-cpu-bind": true, "cpu-request": 2}, coretypes.ErrInsufficientResource},
		{"shrink the bind", node, origin, plugintypes.WorkloadResourceRequest{"keep-cpu-bind": true, "cpu-request": -0.5, "cpu-limit": -0.5}, nil},
		{"no change", node, origin, plugintypes.WorkloadResourceRequest{}, nil},
		{"memory over the node", node, origin, plugintypes.WorkloadResourceRequest{"memory-request": fmt.Sprintf("%v", units.PB), "memory-limit": fmt.Sprintf("%v", units.PB)}, coretypes.ErrInsufficientCapacity},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := cm.CalculateRealloc(ctx, tt.node, tt.origin, tt.req)
			if tt.wantErr == nil {
				assert.NoError(t, err)
				return
			}
			assert.ErrorIs(t, err, tt.wantErr)
		})
	}
}

func TestCalculateRemap(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	nodes := generateNodes(ctx, t, cm, 1, 4, 4*units.GB, 100, 0)
	node := nodes[0]

	resource := plugintypes.NodeResource{
		"cpu_map": map[string]int64{
			"0": 100,
			"1": 100,
		},
	}
	_, err := cm.SetNodeResourceUsage(ctx, node, resource, nil, nil, false, true)
	assert.Nil(t, err)

	workloadsResource := map[string]plugintypes.WorkloadResource{
		"id1": {
			"cpu_request":    1,
			"cpu_limit":      1,
			"memory_request": units.GB,
			"memory_limit":   units.GB,
		},
		"id2": {
			"cpu_request":    1,
			"cpu_limit":      1,
			"memory_request": units.GB,
			"memory_limit":   units.GB,
		},
		"id3": {
			"cpu_map":        types.CPUMap{"0": 100, "1": 100},
			"cpu_request":    2,
			"cpu_limit":      2,
			"memory_request": units.GB,
			"memory_limit":   units.GB,
		},
	}

	_, err = cm.CalculateRemap(ctx, "xxx", workloadsResource)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	r, err := cm.CalculateRemap(ctx, node, workloadsResource)
	assert.Nil(t, err)
	assert.Len(t, r.EngineParamsMap, 2)
	w1 := r.EngineParamsMap["id1"]
	w2 := r.EngineParamsMap["id2"]
	assert.Len(t, w1["cpu_map"], 2)
	assert.Len(t, w2["cpu_map"], 2)

	workloadsResource["id4"] = plugintypes.WorkloadResource{
		"cpu_map":        types.CPUMap{"2": 100, "3": 100},
		"cpu_request":    2,
		"cpu_limit":      2,
		"memory_request": units.GB,
		"memory_limit":   units.GB,
	}

	resource = plugintypes.NodeResource{
		"cpu_map": map[string]int64{
			"0": 100,
			"1": 100,
			"2": 100,
			"3": 100,
		},
	}
	_, err = cm.SetNodeResourceUsage(ctx, node, resource, nil, nil, false, true)
	assert.Nil(t, err)

	r, err = cm.CalculateRemap(ctx, node, workloadsResource)
	assert.Nil(t, err)
	assert.Len(t, r.EngineParamsMap, 2)
	w1 = r.EngineParamsMap["id1"]
	w2 = r.EngineParamsMap["id2"]
	assert.Len(t, w1["cpu_map"], 4)
	assert.Len(t, w2["cpu_map"], 4)
}
