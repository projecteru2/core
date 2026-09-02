package etcdv3

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func TestDeploy(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
		NodeFilter:   &types.NodeFilter{},
	}

	nodeCount, err := m.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	assert.NoError(t, err)
	assert.Equal(t, len(nodeCount), 0)
	key := filepath.Join(common.WorkloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = kvOf(m).Put(ctx, key, "")
	assert.NoError(t, err)
	key = filepath.Join(common.WorkloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	_, err = kvOf(m).Put(ctx, key, "")
	assert.NoError(t, err)
	nodeCount, err = m.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	assert.NoError(t, err)
	assert.Equal(t, nodeCount["node"], 2)
}
