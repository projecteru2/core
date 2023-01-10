package cpumem

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/docker/go-units"
	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
	coretypes "github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestAddNode(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]
	nodeForAdd := "test2"

	req := &plugintypes.NodeResourceRequest{
		"numa-cpu": []string{"0", "1"},
	}

	info := &enginetypes.Info{NCPU: 2, MemTotal: 4 * units.GB}

	// existent node
	_, err := cm.AddNode(ctx, node, req, info)
	assert.Equal(t, err, coretypes.ErrNodeExists)

	// normal case
	r, err := cm.AddNode(ctx, nodeForAdd, req, info)
	assert.Nil(t, err)
	assert.Equal(t, (*r.Capacity)["memory"], int64(4*units.GB*rate/10))
}

func TestRemoveNode(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 1, 2, 4*units.GB, 100, 0)
	node := nodes[0]
	nodeForDel := "test2"

	_, err := cm.RemoveNode(ctx, node)
	assert.Nil(t, err)
	_, err = cm.RemoveNode(ctx, nodeForDel)
	assert.Nil(t, err)
}

func TestGetNodesDeployCapacityWithCPUBind(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 4*units.GB, 100, 0)

	req := &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    0.5,
		"memory-request": "1",
	}

	// non-existent node
	_, err := cm.GetNodesDeployCapacity(ctx, []string{"xxx"}, req)
	assert.True(t, errors.Is(err, coretypes.ErrInvaildCount))

	// normal
	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total >= 1)

	// more cpu
	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    2,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total < 3)

	// more
	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    3,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total < 2)

	// less
	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.True(t, r.Total < 5)

	// complex
	nodes = generateNodes(ctx, t, cm, 1, 4, 12*units.GB, 100, 10)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 14, 12*units.GB, 100, 11)...)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 12, 12*units.GB, 100, 12)...)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 18, 12*units.GB, 100, 13)...)
	nodes = append(nodes, generateNodes(ctx, t, cm, 1, 8, 12*units.GB, 100, 14)...)

	req = &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1.7,
		"memory-request": "1",
	}
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 28)
}

func TestGetNodesDeployCapacityWithMemory(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 2, 2, 1024, 100, 0)

	req := &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    0.1,
		"memory-request": "1024",
	}

	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 2)

	(*req)["memory-request"] = "1025"
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 0)
}

func TestGetNodesDeployCapacityWithMaxShareLimit(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	cm.config.Scheduler.MaxShare = 2
	nodes := generateNodes(ctx, t, cm, 1, 6, 12*units.GB, 100, 0)
	node := nodes[0]

	req := &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1.7,
		"memory-request": "1",
	}

	r, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 2)

	// numa node
	resource := &plugintypes.NodeResource{
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

	(*req)["cpu-request"] = 1.2
	r, err = cm.GetNodesDeployCapacity(ctx, nodes, req)
	assert.Nil(t, err)
	assert.Equal(t, r.Total, 1)
}

func BenchmarkGetNodesCapacity(b *testing.B) {
	b.StopTimer()
	t := &testing.T{}
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	nodes := generateNodes(ctx, t, cm, 10000, 24, 128*units.GB, 100, 0)
	req := &plugintypes.WorkloadResourceRequest{
		"cpu-bind":       true,
		"cpu-request":    1.3,
		"memory-request": "1",
	}
	b.StartTimer()

	for i := 0; i < b.N; i++ {
		_, err := cm.GetNodesDeployCapacity(ctx, nodes, req)
		assert.Nil(b, err)
	}
}
