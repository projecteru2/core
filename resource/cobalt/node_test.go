package cobalt

import (
	"math"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

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

func TestGetNodesDeployCapacityWeightsEveryPlugin(t *testing.T) {
	m, err := New(coretypes.Config{})
	assert.NoError(t, err)
	m.AddPlugins(
		newCapacityPlugin(t, "cpumem", map[string]*plugintypes.NodeDeployCapacity{
			"n1": {Capacity: 10, Rate: 0.5, Usage: 0.5, Weight: 100},
		}),
		newCapacityPlugin(t, "storage", map[string]*plugintypes.NodeDeployCapacity{
			"n1": {Capacity: 10, Rate: 0.1, Usage: 0.1, Weight: 1},
		}),
	)

	for range 50 {
		resp, _, err := m.GetNodesDeployCapacity(t.Context(), []string{"n1"}, resourcetypes.Resources{})
		assert.NoError(t, err)
		assert.InDelta(t, (0.5*100+0.1*1)/101, resp["n1"].Rate, 1e-9)
		assert.InDelta(t, (0.5*100+0.1*1)/101, resp["n1"].Usage, 1e-9)
	}
}

func TestRemoveNodeRollbackRestoresNonWhitelistedPlugins(t *testing.T) {
	capacity := plugintypes.NodeResource{"memory": int64(1024)}
	usage := plugintypes.NodeResource{"memory": int64(512)}

	whitelisted := pluginmocks.NewPlugin(t)
	whitelisted.On("Name").Return("cpumem").Maybe()
	whitelisted.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).
		Return(&plugintypes.GetNodeResourceInfoResponse{Capacity: capacity, Usage: usage}, nil)
	whitelisted.On("RemoveNode", mock.Anything, mock.Anything).Return(nil, errors.New("etcd unavailable"))

	other := pluginmocks.NewPlugin(t)
	other.On("Name").Return("storage").Maybe()
	other.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything).
		Return(&plugintypes.GetNodeResourceInfoResponse{Capacity: capacity, Usage: usage}, nil)
	other.On("RemoveNode", mock.Anything, mock.Anything).Return(&plugintypes.RemoveNodeResponse{}, nil)

	restored := make(chan plugintypes.NodeResource, 1)
	other.On("SetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			restored <- args.Get(2).(plugintypes.NodeResource)
		}).
		Return(&plugintypes.SetNodeResourceInfoResponse{}, nil)

	m, err := New(coretypes.Config{
		GlobalTimeout:  time.Minute,
		ResourcePlugin: coretypes.ResourcePluginConfig{Whitelist: []string{"cpumem"}},
	})
	assert.NoError(t, err)
	m.AddPlugins(whitelisted, other)

	assert.Error(t, m.RemoveNode(t.Context(), "n1"))
	assert.Equal(t, capacity, <-restored)
}

func newCapacityPlugin(t *testing.T, name string, capacities map[string]*plugintypes.NodeDeployCapacity) *pluginmocks.Plugin {
	p := pluginmocks.NewPlugin(t)
	p.On("Name").Return(name).Maybe()
	resp := &plugintypes.GetNodesDeployCapacityResponse{NodeDeployCapacityMap: capacities}
	p.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
	return p
}
