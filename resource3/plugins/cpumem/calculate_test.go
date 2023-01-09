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
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100)
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
