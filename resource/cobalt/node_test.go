package cobalt

import (
	"math"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	pluginmocks "github.com/projecteru2/core/resource/plugins/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	coretypes "github.com/projecteru2/core/types"
)

func TestGetNodesDeployCapacityTotalSaturates(t *testing.T) {
	m, err := New(coretypes.Config{})
	assert.NoError(t, err)
	m.AddPlugins(newCapacityPlugin(t, "cpumem", map[string]*plugintypes.NodeDeployCapacity{
		"unbounded": {Capacity: math.MaxInt, Weight: 1},
		"bounded":   {Capacity: 5, Weight: 1},
	}))

	for range 50 {
		_, total, err := m.GetNodesDeployCapacity(t.Context(), []string{"unbounded", "bounded"}, resourcetypes.Resources{})
		assert.NoError(t, err)
		assert.Equal(t, math.MaxInt, total)
	}
}

func newCapacityPlugin(t *testing.T, name string, capacities map[string]*plugintypes.NodeDeployCapacity) *pluginmocks.Plugin {
	p := pluginmocks.NewPlugin(t)
	p.On("Name").Return(name).Maybe()
	resp := &plugintypes.GetNodesDeployCapacityResponse{NodeDeployCapacityMap: capacities}
	p.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
	return p
}
