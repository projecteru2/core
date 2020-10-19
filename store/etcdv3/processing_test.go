package etcdv3

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
)

func TestProcessing(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
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
	assert.NoError(t, m.SaveProcessing(ctx, opts, nodeInfo.Name, nodeInfo.Deploy))
	// create again
	assert.Error(t, m.SaveProcessing(ctx, opts, nodeInfo.Name, nodeInfo.Deploy))
	// update
	assert.NoError(t, m.UpdateProcessing(ctx, opts, nodeInfo.Name, 8))

	sis := []types.StrategyInfo{{Nodename: "node"}}
	err := m.doLoadProcessing(ctx, opts, sis)
	assert.NoError(t, err)
	assert.Equal(t, sis[0].Count, 8)
	// delete
	assert.NoError(t, m.DeleteProcessing(ctx, opts, nodeInfo.Name))
}
