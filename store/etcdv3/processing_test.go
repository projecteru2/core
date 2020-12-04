package etcdv3

import (
	"context"
	"testing"

	"github.com/projecteru2/core/strategy"
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

	// not exists
	assert.Error(t, m.UpdateProcessing(ctx, opts, "node", 8))
	// create
	assert.NoError(t, m.SaveProcessing(ctx, opts, "node", 10))
	// create again
	assert.Error(t, m.SaveProcessing(ctx, opts, "node", 10))
	// update
	assert.NoError(t, m.UpdateProcessing(ctx, opts, "node", 8))

	sis := []strategy.Info{{Nodename: "node"}}
	err := m.doLoadProcessing(ctx, opts, sis)
	assert.NoError(t, err)
	assert.Equal(t, sis[0].Count, 8)
	// delete
	assert.NoError(t, m.DeleteProcessing(ctx, opts, "node"))
}
