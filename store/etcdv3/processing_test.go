package etcdv3

import (
	"context"
	"testing"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestProcessing(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	processing := &types.Processing{
		Appname:   "app",
		Entryname: "entry",
		Nodename:  "node",
		Ident:     "abc",
	}

	// create
	assert.NoError(t, m.CreateProcessing(ctx, processing, 10))
	// create again
	assert.Error(t, m.CreateProcessing(ctx, processing, 10))
	assert.NoError(t, m.AddWorkload(ctx, &types.Workload{Name: "a_b_c"}, processing))

	nodeCount, err := m.doLoadProcessing(ctx, processing.Appname, processing.Entryname)
	assert.NoError(t, err)
	assert.Equal(t, nodeCount["node"], 9)
	// delete
	assert.NoError(t, m.DeleteProcessing(ctx, processing))
}
