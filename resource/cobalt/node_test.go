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
	m := New(coretypes.Config{})
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
	m := New(coretypes.Config{})
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

	m := New(coretypes.Config{
		GlobalTimeout:  time.Minute,
		ResourcePlugin: coretypes.ResourcePluginConfig{Whitelist: []string{"cpumem"}},
	})
	m.AddPlugins(whitelisted, other)

	assert.Error(t, m.RemoveNode(t.Context(), "n1"))
	assert.Equal(t, capacity, <-restored)
}

func TestSetNodeResourceCapacityReturnsSuccessfulChange(t *testing.T) {
	beforeResource := plugintypes.NodeResource{"memory": int64(1024)}
	afterResource := plugintypes.NodeResource{"memory": int64(2048)}

	plugin := pluginmocks.NewPlugin(t)
	plugin.On("Name").Return("cpumem").Maybe()
	plugin.On("SetNodeResourceCapacity", mock.Anything, "n1", mock.Anything, afterResource, false, true).
		Return(&plugintypes.SetNodeResourceCapacityResponse{Before: beforeResource, After: afterResource}, nil).
		Once()

	m := New(coretypes.Config{GlobalTimeout: time.Minute})
	m.AddPlugins(plugin)

	before, after, err := m.SetNodeResourceCapacity(
		t.Context(),
		"n1",
		resourcetypes.Resources{},
		resourcetypes.Resources{"cpumem": afterResource},
		false,
		true,
	)
	assert.NoError(t, err)
	assert.Equal(t, beforeResource, before["cpumem"])
	assert.Equal(t, afterResource, after["cpumem"])
}

func TestGetNodesResourceInfoMergesEveryPlugin(t *testing.T) {
	m := New(coretypes.Config{})
	m.AddPlugins(
		newResourceInfoPlugin(t, "cpumem", map[string]*plugintypes.NodeResourceInfo{
			"n1": {Capacity: plugintypes.NodeResource{"cpu": 8}, Usage: plugintypes.NodeResource{"cpu": 1}},
			"n2": {Capacity: plugintypes.NodeResource{"cpu": 4}, Usage: plugintypes.NodeResource{"cpu": 0}},
		}),
		newResourceInfoPlugin(t, "storage", map[string]*plugintypes.NodeResourceInfo{
			"n1": {Capacity: plugintypes.NodeResource{"storage": 100}, Usage: plugintypes.NodeResource{"storage": 10}},
		}),
	)

	infos, err := m.GetNodesResourceInfo(t.Context(), []string{"n1", "n2"})
	assert.NoError(t, err)
	assert.Equal(t, plugintypes.NodeResource{"cpu": 8}, infos["n1"].Capacity["cpumem"])
	assert.Equal(t, plugintypes.NodeResource{"storage": 10}, infos["n1"].Usage["storage"])
	assert.Equal(t, plugintypes.NodeResource{"cpu": 4}, infos["n2"].Capacity["cpumem"])
	assert.NotContains(t, infos["n2"].Capacity, "storage")
}

func TestGetNodesResourceInfoKeepsTheHealthyPlugins(t *testing.T) {
	m := New(coretypes.Config{})
	broken := pluginmocks.NewPlugin(t)
	broken.On("Name").Return("gpu").Maybe()
	broken.On("GetNodesResourceInfo", mock.Anything, mock.Anything).Return(nil, coretypes.ErrMockError)
	m.AddPlugins(
		newResourceInfoPlugin(t, "cpumem", map[string]*plugintypes.NodeResourceInfo{"n1": {Capacity: plugintypes.NodeResource{"cpu": 8}}}),
		broken,
	)

	infos, err := m.GetNodesResourceInfo(t.Context(), []string{"n1"})
	assert.ErrorIs(t, err, coretypes.ErrMockError)
	assert.Equal(t, plugintypes.NodeResource{"cpu": 8}, infos["n1"].Capacity["cpumem"])
}

func newCapacityPlugin(t *testing.T, name string, capacities map[string]*plugintypes.NodeDeployCapacity) *pluginmocks.Plugin {
	p := pluginmocks.NewPlugin(t)
	p.On("Name").Return(name).Maybe()
	resp := &plugintypes.GetNodesDeployCapacityResponse{NodeDeployCapacityMap: capacities}
	p.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(resp, nil)
	return p
}

func newResourceInfoPlugin(t *testing.T, name string, infos map[string]*plugintypes.NodeResourceInfo) *pluginmocks.Plugin {
	p := pluginmocks.NewPlugin(t)
	p.On("Name").Return(name).Maybe()
	resp := &plugintypes.GetNodesResourceInfoResponse{NodeResourceInfoMap: infos}
	p.On("GetNodesResourceInfo", mock.Anything, mock.Anything).Return(resp, nil)
	return p
}
