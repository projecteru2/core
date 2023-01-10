package cpumem

import (
	"context"
	"fmt"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/docker/go-units"
	"github.com/projecteru2/core/resource3/plugins/cpumem/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestCalculateDeploy(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	// invalid opts
	req := &plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"cpu-request": -1,
	}
	_, err := cm.CalculateDeploy(ctx, node, 100, req)
	assert.True(t, errors.Is(err, types.ErrInvalidCPU))

	// non-existent node
	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"cpu-request": 1,
	}
	_, err = cm.CalculateDeploy(ctx, "xxx", 100, req)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	// cpu bind
	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"cpu-request": 1.1,
	}
	_, err = cm.CalculateDeploy(ctx, node, 1, req)
	assert.Nil(t, err)

	// cpu bind & insufficient resource
	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"cpu-request": 2.2,
	}
	_, err = cm.CalculateDeploy(ctx, node, 1, req)
	assert.True(t, errors.Is(err, coretypes.ErrInsufficientCapacity))

	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"cpu-request": 1,
	}
	_, err = cm.CalculateDeploy(ctx, node, 3, req)
	assert.True(t, errors.Is(err, coretypes.ErrInsufficientCapacity))

	// alloc by memory
	req = &plugintypes.WorkloadResourceRequest{
		"memory-request": fmt.Sprintf("%v", units.GB),
	}
	_, err = cm.CalculateDeploy(ctx, node, 1, req)
	assert.Nil(t, err)

	// alloc by memory & insufficient cpu
	req = &plugintypes.WorkloadResourceRequest{
		"memory-request": fmt.Sprintf("%v", units.GB),
		"cpu-request":    1000,
	}
	_, err = cm.CalculateDeploy(ctx, node, 1, req)
	assert.True(t, errors.Is(err, coretypes.ErrInsufficientCapacity))

	// alloc by memory & insufficient mem
	req = &plugintypes.WorkloadResourceRequest{
		"memory-request": fmt.Sprintf("%v", 5*units.GB),
		"cpu-request":    1,
	}
	_, err = cm.CalculateDeploy(ctx, node, 1, req)
	assert.True(t, errors.Is(err, coretypes.ErrInsufficientCapacity))

	// mem_request == 0
	req = &plugintypes.WorkloadResourceRequest{
		"memory-request": "0",
		"cpu-request":    1,
	}
	_, err = cm.CalculateDeploy(ctx, node, 1, req)
	assert.Nil(t, err)

	// numa node
	resource := &plugintypes.NodeResource{
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

	_, err = cm.SetNodeResourceCapacity(ctx, node, resource, nil, false, true)
	assert.Nil(t, err)

	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":    true,
		"memory":      fmt.Sprintf("%v", units.GB),
		"cpu-request": 1.3,
	}
	r, err := cm.CalculateDeploy(ctx, node, 1, req)
	assert.Nil(t, err)
	assert.NotNil(t, r.WorkloadsResource)
}

func TestCalculateRealloc(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]

	// numa node
	resource := &plugintypes.NodeResource{
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
	assert.Nil(t, err)

	origin := &plugintypes.WorkloadResource{}
	req := &plugintypes.WorkloadResourceRequest{}

	// non-existent node
	_, err = cm.CalculateRealloc(ctx, "xxx", origin, req)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	// invalid resource opts
	origin = &plugintypes.WorkloadResource{
		"cpu_request":    1,
		"cpu_limit":      1,
		"memory_request": units.GB,
		"memory_limit":   units.GB,
		"cpu_map":        types.CPUMap{"0": 100},
		"numa_memory":    types.NUMAMemory{"0": units.GB},
		"numa_node":      "0",
	}
	req = &plugintypes.WorkloadResourceRequest{
		"keep-cpu-bind": true,
		"cpu-request":   -3,
	}
	_, err = cm.CalculateRealloc(ctx, node, origin, req)
	assert.True(t, errors.Is(err, types.ErrInvalidCPU))

	// insufficient cpu
	req = &plugintypes.WorkloadResourceRequest{
		"keep-cpu-bind": true,
		"cpu-request":   2,
	}
	_, err = cm.CalculateRealloc(ctx, node, origin, req)
	assert.True(t, errors.Is(err, coretypes.ErrInsufficientResource))

	// normal case (with cpu-bind)
	req = &plugintypes.WorkloadResourceRequest{
		"keep-cpu-bind": true,
		"cpu-request":   -0.5,
		"cpu-limit":     -0.5,
	}
	_, err = cm.CalculateRealloc(ctx, node, origin, req)
	assert.Nil(t, err)

	// normal case (without cpu-bind)
	req = &plugintypes.WorkloadResourceRequest{}
	_, err = cm.CalculateRealloc(ctx, node, origin, req)
	assert.Nil(t, err)

	// insufficient mem
	req = &plugintypes.WorkloadResourceRequest{
		"memory-request": fmt.Sprintf("%v", units.PB),
		"memory-limit":   fmt.Sprintf("%v", units.PB),
	}
	_, err = cm.CalculateRealloc(ctx, node, origin, req)
	assert.True(t, errors.Is(err, coretypes.ErrInsufficientCapacity))
}

func TestCalculateRemap(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 1, 4, 4*units.GB, 100, 0)
	node := nodes[0]

	resource := &plugintypes.NodeResource{
		"cpu_map": map[string]int64{
			"0": 100,
			"1": 100,
		},
	}
	_, err := cm.SetNodeResourceUsage(ctx, node, resource, nil, nil, false, true)
	assert.Nil(t, err)

	workloadsResource := map[string]*plugintypes.WorkloadResource{
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

	// non-existent node
	_, err = cm.CalculateRemap(ctx, "xxx", workloadsResource)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	// normal case
	r, err := cm.CalculateRemap(ctx, node, workloadsResource)
	assert.Nil(t, err)
	assert.Len(t, r.EngineParamsMap, 2)
	w1 := r.EngineParamsMap["id1"]
	w2 := r.EngineParamsMap["id2"]
	assert.Len(t, (*w1)["CPUMap"], 2)
	assert.Len(t, (*w2)["CPUMap"], 2)

	// empty share cpu map
	workloadsResource["id4"] = &plugintypes.WorkloadResource{
		"cpu_map":        types.CPUMap{"2": 100, "3": 100},
		"cpu_request":    2,
		"cpu_limit":      2,
		"memory_request": units.GB,
		"memory_limit":   units.GB,
	}

	resource = &plugintypes.NodeResource{
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
	assert.Len(t, (*w1)["CPUMap"], 4)
	assert.Len(t, (*w2)["CPUMap"], 4)
}
