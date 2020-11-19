package calcium

import (
	"context"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestCalculateCapacity(t *testing.T) {
	c := NewTestCluster()
	scheduler.InitSchedulerV1(c.scheduler)
	ctx := context.Background()
	store := c.store.(*storemocks.Store)
	engine := &enginemocks.API{}

	// pod1 := &types.Pod{Name: "p1"}
	node1 := &types.Node{
		Name:   "n1",
		Engine: engine,
		CPU:    types.CPUMap{"0": 100, "1": 100},
	}
	store.On("GetNode", mock.Anything, mock.Anything).Return(node1, nil)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// failed by wrong resource
	opts := &types.DeployOptions{
		ResourceOpts: types.ResourceOptions{
			CPUBind:         true,
			CPUQuotaRequest: 0,
		},
		DeployStrategy: strategy.Auto,
		Nodenames:      []string{"n1"},
	}
	_, err := c.CalculateCapacity(ctx, opts)
	assert.Error(t, err)
	opts.ResourceOpts.CPUBind = false
	opts.ResourceOpts.CPUQuotaRequest = 0.5
	opts.Count = 5
	sched := c.scheduler.(*schedulermocks.Scheduler)
	// define nodesInfo
	nodesInfo := []types.NodeInfo{
		{
			Name:     "n1",
			MemCap:   100,
			Deploy:   5,
			Capacity: 10,
			Count:    1,
		},
	}
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, 5, nil)
	sched.On("SelectStorageNodes", mock.Anything, mock.Anything).Return(nodesInfo, 5, nil)
	sched.On("SelectVolumeNodes", mock.Anything, mock.Anything).Return(nodesInfo, nil, 5, nil)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	r, err := c.CalculateCapacity(ctx, opts)
	assert.NoError(t, err)
	assert.Equal(t, r.Total, 5)
}
