package etcdv3

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestProcessing(t *testing.T) {
	etcd := InitCluster(t)
	defer AfterTest(t, etcd)
	m := NewMercury(t, etcd.RandClient())
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
	}
	nodeInfo := types.NodeInfo{Name: "node", Deploy: 10}

	// not exists
	assert.Error(t, m.UpdateProcessing(ctx, opts, nodeInfo.Name, 8))
	// create
	assert.NoError(t, m.SaveProcessing(ctx, opts, nodeInfo))
	// create again
	assert.Error(t, m.SaveProcessing(ctx, opts, nodeInfo))
	// update
	assert.NoError(t, m.UpdateProcessing(ctx, opts, nodeInfo.Name, 8))

	nodesInfo, err := m.loadProcessing(ctx, opts, []types.NodeInfo{nodeInfo})
	assert.NoError(t, err)
	assert.Equal(t, len(nodesInfo), 1)
	assert.Equal(t, nodesInfo[0].Count, 8)
	// delete
	assert.NoError(t, m.DeleteProcessing(ctx, opts, nodeInfo))
}
