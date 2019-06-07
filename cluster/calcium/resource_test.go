package calcium

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/cluster"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestPodResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	podname := "testpod"
	nodename := "testnode"
	store := &storemocks.Store{}
	c.store = store
	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Once()
	_, err := c.PodResource(ctx, podname)
	assert.Error(t, err)
	node := &types.Node{
		Name:       nodename,
		CPU:        types.CPUMap{"0": 0, "1": 10},
		MemCap:     2,
		InitCPU:    types.CPUMap{"0": 100, "1": 100},
		InitMemCap: 6,
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	// failed by ListNodeContainers
	store.On("ListNodeContainers", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Once()
	_, err = c.PodResource(ctx, podname)
	assert.Error(t, err)
	containers := []*types.Container{
		{
			Memory: 1,
			CPU:    types.CPUMap{"0": 100, "1": 30},
			Quota:  1.3,
		},
		{
			Memory: 2,
			CPU:    types.CPUMap{"1": 50},
			Quota:  0.5,
		},
	}
	store.On("ListNodeContainers", mock.Anything, mock.Anything).Return(containers, nil)
	// success
	r, err := c.PodResource(ctx, podname)
	assert.NoError(t, err)
	assert.Len(t, r.CPUPercent, 1)
	assert.Len(t, r.MEMPercent, 1)
	assert.False(t, r.Diff[nodename])
	assert.NotEmpty(t, r.Detail[nodename])
}

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
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Once()
	_, err := c.doAllocResource(ctx, opts)
	assert.Error(t, err)

	lock := &dummyLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	// test get by pod and labels and failed because node not available
	n1 := "n2"
	n2 := "n2"
	nodes := []*types.Node{
		{
			Name:      n1,
			Available: false,
			Labels:    map[string]string{"test": "1"},
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
		{
			Name:      n2,
			Available: true,
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nil, types.ErrBadPodType).Once()
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	store.On("GetNodesByPod", mock.Anything, mock.Anything).Return(nodes, nil)
	opts.NodeLabels = map[string]string{"test": "1"}
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// get nodes by name failed
	opts.NodeLabels = nil
	opts.Nodename = n2
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// get nodes by name success
	store.On("GetNode",
		mock.Anything,
		mock.Anything,
		mock.Anything).Return(nodes[1], nil)
	// define nodesInfo
	nodesInfo := []types.NodeInfo{
		{
			Name:     n2,
			CPUMap:   types.CPUMap{"0": 100},
			MemCap:   100,
			Count:    1,  // 1 exists
			Deploy:   3,  // deploy 1
			Capacity: 10, // can deploy 10
		},
	}
	nodeCPUPlans := map[string][]types.CPUMap{
		n2: {
			{"0": 10},
			{"0": 10},
			{"0": 10},
			{"0": 10},
			{"0": 10},
		},
	}
	// mock MakeDeployStatus
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// wrong podType
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nil)
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// mock Schedulers
	sched := &schedulermocks.Scheduler{}
	c.scheduler = sched
	// wrong select
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrInsufficientMEM).Once()
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	//cpu select
	total := 3
	sched.On("SelectCPUNodes", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nodeCPUPlans, total, nil)
	// wrong DeployMethod
	opts.CPUBind = true
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// other methods
	sched.On("CommonDivision", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrInsufficientRes)
	opts.DeployMethod = cluster.DeployAuto
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	sched.On("GlobalDivision", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrInsufficientRes)
	opts.DeployMethod = cluster.DeployGlobal
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	sched.On("EachDivision", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrInsufficientRes)
	opts.DeployMethod = cluster.DeployEach
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// fill division but no nodes failed
	sched.On("FillDivision", mock.Anything, mock.Anything, mock.Anything).Return([]types.NodeInfo{}, nil).Once()
	opts.DeployMethod = cluster.DeployFill
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// fill division but UpdateNodeResource failed
	sched.On("FillDivision", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, nil)
	store.On("UpdateNodeResource",
		mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything,
	).Return(types.ErrNoETCD).Once()
	_, err = c.doAllocResource(ctx, opts)
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
	_, err = c.doAllocResource(ctx, opts)
	assert.Error(t, err)
	// bind process
	store.On("SaveProcessing",
		mock.Anything, mock.Anything, mock.Anything,
	).Return(nil)
	nsi, err := c.doAllocResource(ctx, opts)
	assert.NoError(t, err)
	assert.Len(t, nsi, 1)
	assert.Equal(t, nsi[0].Name, n2)
	// stupid race condition
}
