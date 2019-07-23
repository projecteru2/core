package etcdv3

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/types"
)

func TestDeploy(t *testing.T) {
	m := NewMercury(t)
	defer m.TerminateEmbededStorage()
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
	}
	nodeInfo := types.NodeInfo{Name: "node"}

	// no container deployed
	nodesInfo, err := m.MakeDeployStatus(ctx, opts, []types.NodeInfo{nodeInfo})
	assert.NoError(t, err)
	assert.Equal(t, len(nodesInfo), 1)
	assert.Equal(t, nodesInfo[0].Name, nodeInfo.Name)
	// have containers
	key := filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	key = filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	nodesInfo, err = m.MakeDeployStatus(ctx, opts, []types.NodeInfo{nodeInfo})
	assert.NoError(t, err)
	assert.Equal(t, len(nodesInfo), 1)
	assert.Equal(t, nodesInfo[0].Name, nodeInfo.Name)
	assert.Equal(t, nodesInfo[0].Count, 2)
}
