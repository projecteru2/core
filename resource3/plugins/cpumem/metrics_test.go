package cpumem

import (
	"context"
	"testing"

	enginetypes "github.com/projecteru2/core/engine/types"
	plugintypes "github.com/projecteru2/core/resource3/plugins/types"
	"github.com/stretchr/testify/assert"
)

func TestGetMetricsDescription(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	md, err := cm.GetMetricsDescription(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, md)
	assert.Len(t, *md, 4)
}

func TestGetMetrics(t *testing.T) {
	ctx := context.Background()
	cm := initCPUMEM(ctx, t)
	_, err := cm.GetMetrics(ctx, "", "")
	assert.Error(t, err)

	req := &plugintypes.NodeResourceRequest{
		"cpu":         2,
		"share":       0,
		"memory":      "1gb",
		"numa-cpu":    []string{"0", "1"},
		"numa-memory": []string{"512mb", "512mb"},
		"xxxx":        "uuuu",
	}

	info := &enginetypes.Info{NCPU: 8, MemTotal: 2048}
	r, err := cm.AddNode(ctx, "test", req, info)
	assert.NoError(t, err)
	assert.NotNil(t, r)
}
