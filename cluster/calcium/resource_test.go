package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/cluster"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestAllocResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	podname := "testpod"
	opts := &types.DeployOptions{
		Podname: podname,
	}
	config := types.Config{
		LockTimeout: 3,
	}
	store := &storemocks.Store{}
	c.config = config
	c.store = store

	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.allocResource(ctx, opts, "")
	assert.Error(t, err)

	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// test get by pod and labels and failed because node not avaliable
	n1 := "n2"
	n2 := "n2"
	nodes := []*types.Node{
		&types.Node{
			Name:      n1,
			Available: false,
			Labels:    map[string]string{"test": "1"},
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
		&types.Node{
			Name:      n2,
			Available: true,
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Once()
	_, err = c.allocResource(ctx, opts, "")
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nodes, nil)
	opts.NodeLabels = map[string]string{"test": "1"}
	_, err = c.allocResource(ctx, opts, "")
	assert.Error(t, err)
	// get nodes by name failed
	opts.NodeLabels = nil
	opts.Nodename = n2
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.allocResource(ctx, opts, "")
	assert.Error(t, err)
	// get nodes by name success
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nodes[1], nil)
	// define nodesInfo
	nodesInfo := []types.NodeInfo{
		types.NodeInfo{
			Name: n2,
			CPUAndMem: types.CPUAndMem{
				CPUMap: types.CPUMap{"0": 100},
				MemCap: 100,
			},
			CPUs:     1,
			Count:    1,  // 1 exists
			Deploy:   3,  // deploy 1
			Capacity: 10, // can deploy 10
		},
	}
	nodeCPUPlans := map[string][]types.CPUMap{
		n2: []types.CPUMap{
			types.CPUMap{"0": 10},
			types.CPUMap{"0": 10},
			types.CPUMap{"0": 10},
			types.CPUMap{"0": 10},
			types.CPUMap{"0": 10},
		},
	}
	// mock MakeDeployStatus
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.allocResource(ctx, opts, "")
	assert.Error(t, err)
	// wrong podType
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nil)
	_, err = c.allocResource(ctx, opts, "")
	assert.Error(t, err)
	// mock Schedulers
	sched := &schedulermocks.Scheduler{}
	c.scheduler = sched
	// wrong select
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrInsufficientMEM).Once()
	_, err = c.allocResource(ctx, opts, scheduler.MEMORY_PRIOR)
	assert.Error(t, err)
	//cpu select
	total := 3
	sched.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nodeCPUPlans, total, nil)
	// wrong DeployMethod
	_, err = c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.Error(t, err)
	// other methods
	sched.On("CommonDivision", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrInsufficientRes)
	opts.DeployMethod = cluster.DeployAuto
	_, err = c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.Error(t, err)
	sched.On("EachDivision", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrInsufficientRes)
	opts.DeployMethod = cluster.DeployEach
	_, err = c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.Error(t, err)
	// fill division but no nodes failed
	sched.On("FillDivision", mock.Anything, mock.Anything, mock.Anything).Return([]types.NodeInfo{}, nil).Once()
	opts.DeployMethod = cluster.DeployFill
	_, err = c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.Error(t, err)
	// fill division but UpdateNodeResource failed
	sched.On("FillDivision", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nil)
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Once()
	_, err = c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.Error(t, err)
	// fill division sucessed
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	// bind process failed
	store.On("SaveProcessing",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Once()
	_, err = c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.Error(t, err)
	// bind process
	store.On("SaveProcessing",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	nsi, err := c.allocResource(ctx, opts, scheduler.CPU_PRIOR)
	assert.NoError(t, err)
	assert.Len(t, nsi, 1)
	assert.Equal(t, nsi[0].Name, n2)
}
