package cpumem

import (
	"testing"

	"github.com/docker/go-units"
	"github.com/stretchr/testify/assert"

	plugintypes "github.com/projecteru2/core/resource/plugins/types"
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
	_, err := cm.GetMetrics(ctx, []plugintypes.NodeRef{{}})
	assert.Error(t, err)

	nodes := generateNodes(ctx, t, cm, 2, 2, units.GB, 100, -1)
	metrics, err := cm.GetMetrics(ctx, []plugintypes.NodeRef{{Podname: "testpod", Nodename: nodes[0]}, {Podname: "testpod", Nodename: nodes[1]}})
	assert.NoError(t, err)
	assert.Len(t, *metrics, 10)
}
