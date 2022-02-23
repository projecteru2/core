package etcdv3

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestDeploy(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	opts := &types.DeployOptions{
		Name:         "app",
		Entrypoint:   &types.Entrypoint{Name: "entry"},
		ProcessIdent: "abc",
	}

	// no workload deployed
	nodeCount, err := m.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	assert.NoError(t, err)
	assert.Equal(t, len(nodeCount), 0)
	// have workloads
	key := filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	key = filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	nodeCount, err = m.GetDeployStatus(ctx, opts.Name, opts.Entrypoint.Name)
	assert.NoError(t, err)
	assert.Equal(t, nodeCount["node"], 2)
}
