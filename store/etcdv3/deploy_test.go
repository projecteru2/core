package etcdv3

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/projecteru2/core/strategy"
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
	sis := []strategy.Info{
		{Nodename: "node"},
	}

	// no workload deployed
	err := m.MakeDeployStatus(ctx, opts.Name, opts.Entrypoint.Name, sis)
	assert.NoError(t, err)
	assert.Equal(t, len(sis), 1)
	// have workloads
	key := filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	key = filepath.Join(workloadDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	err = m.MakeDeployStatus(ctx, opts.Name, opts.Entrypoint.Name, sis)
	assert.NoError(t, err)
	assert.Equal(t, len(sis), 1)
	assert.Equal(t, sis[0].Nodename, "node")
	assert.Equal(t, sis[0].Count, 2)
}
