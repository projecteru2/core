package cpumem

import (
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"
)

func TestGetMetricsDescription(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	md, err := cm.GetMetricsDescription(ctx)
	assert.NoError(t, err)
	assert.NotNil(t, md)
	assert.Len(t, *md, 4)
}

func TestGetMetrics(t *testing.T) {
	ctx := t.Context()
	cm := initCPUMEM(t)
	_, err := cm.GetMetrics(ctx, "", "")
	assert.Error(t, err)

	nodes := generateNodes(ctx, t, cm, 1, 2, units.GB, 100, -1)
	_, err = cm.GetMetrics(ctx, "testpod", nodes[0])
	assert.NoError(t, err)
}
