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
	sis := []types.StrategyInfo{
		{Nodename: "node"},
	}

	// no container deployed
	err := m.MakeDeployStatus(ctx, opts, sis)
	assert.NoError(t, err)
	assert.Equal(t, len(sis), 1)
	// have containers
	key := filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id1")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	key = filepath.Join(containerDeployPrefix, opts.Name, opts.Entrypoint.Name, "node", "id2")
	_, err = m.Put(ctx, key, "")
	assert.NoError(t, err)
	err = m.MakeDeployStatus(ctx, opts, sis)
	assert.NoError(t, err)
	assert.Equal(t, len(sis), 1)
	assert.Equal(t, sis[0].Nodename, "node")
	assert.Equal(t, sis[0].Count, 2)
}
