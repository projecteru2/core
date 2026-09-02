package cobalt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/resource/plugins"
	pluginmocks "github.com/projecteru2/core/resource/plugins/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestRemapLeavesOutAPluginWithoutTheVerb(t *testing.T) {
	m := New(coretypes.Config{})
	cpumem := pluginmocks.NewPlugin(t)
	cpumem.On("Name").Return("cpumem").Maybe()
	cpumem.On("CalculateRemap", mock.Anything, mock.Anything, mock.Anything).Return(&plugintypes.CalculateRemapResponse{
		EngineParamsMap: map[string]resourcetypes.RawParams{"w1": {"cpus": "1-3"}},
	}, nil)
	gpu := pluginmocks.NewPlugin(t)
	gpu.On("Name").Return("gpu").Maybe()
	gpu.On("CalculateRemap", mock.Anything, mock.Anything, mock.Anything).Return(nil, plugins.ErrVerbNotSupported)
	m.AddPlugins(cpumem, gpu)

	params, err := m.Remap(t.Context(), "n1", []*coretypes.Workload{{ID: "w1"}})
	assert.NoError(t, err)
	assert.Equal(t, resourcetypes.Resources{"cpumem": {"cpus": "1-3"}}, params["w1"])
}
