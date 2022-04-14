package models

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/resources/cpumem/types"
)

func TestGetMostIdleNode(t *testing.T) {
	ctx := context.Background()
	cpuMem := newTestCPUMem(t)

	infos := generateNodeResourceInfos(t, 2, 2, 2*units.GiB, 100)
	infos[0].Usage = &types.NodeResourceArgs{
		CPU:    0,
		CPUMap: types.CPUMap{},
		Memory: 0,
	}
	infos[1].Usage = &types.NodeResourceArgs{
		CPU:    1,
		CPUMap: types.CPUMap{},
		Memory: 100,
	}

	nodes := []string{}

	for i, info := range infos {
		nodeName := fmt.Sprintf("node%d", i)
		assert.Nil(t, cpuMem.doSetNodeResourceInfo(context.Background(), nodeName, info))
		nodes = append(nodes, nodeName)
	}

	node, _, err := cpuMem.GetMostIdleNode(ctx, nodes)
	assert.Nil(t, err)
	assert.Equal(t, node, "node0")

	nodes = append(nodes, "node-x")
	node, _, err = cpuMem.GetMostIdleNode(ctx, nodes)
	assert.NotNil(t, err)
}
