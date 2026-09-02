package cobalt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/resource/plugins"
	pluginmocks "github.com/projecteru2/core/resource/plugins/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestGetNodesMetricsLeavesOutAPluginWithoutTheVerb(t *testing.T) {
	m := New(coretypes.Config{})
	cpumem := pluginmocks.NewPlugin(t)
	cpumem.On("Name").Return("cpumem").Maybe()
	cpumem.On("GetMetrics", mock.Anything, mock.Anything).Return(&plugintypes.GetMetricsResponse{{Name: "cpu_used"}}, nil)
	gpu := pluginmocks.NewPlugin(t)
	gpu.On("Name").Return("gpu").Maybe()
	gpu.On("GetMetrics", mock.Anything, mock.Anything).Return(nil, plugins.ErrVerbNotSupported)
	m.AddPlugins(cpumem, gpu)

	metrics, err := m.GetNodesMetrics(t.Context(), []*coretypes.Node{{NodeMeta: coretypes.NodeMeta{Name: "n1", Podname: "p"}}})
	assert.NoError(t, err)
	assert.Len(t, metrics, 1)
}
