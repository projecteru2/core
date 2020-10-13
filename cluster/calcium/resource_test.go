package calcium

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	"github.com/projecteru2/core/scheduler/resources"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
)

func TestPodResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	podname := "testpod"
	nodename := "testnode"
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	// failed by GetNodesByPod
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := c.PodResource(ctx, podname)
	assert.Error(t, err)
	node := &types.Node{
		Name:           nodename,
		CPU:            types.CPUMap{"0": 0, "1": 10},
		MemCap:         2,
		InitCPU:        types.CPUMap{"0": 100, "1": 100},
		InitMemCap:     6,
		InitStorageCap: 10,
	}
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*types.Node{node}, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	// failed by ListNodeContainers
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.PodResource(ctx, podname)
	assert.Error(t, err)
	containers := []*types.Container{
		{
			Memory: 1,
			CPU:    types.CPUMap{"0": 100, "1": 30},
			Quota:  1.3,
		},
		{
			Memory:  2,
			CPU:     types.CPUMap{"1": 50},
			Quota:   0.5,
			Storage: 1,
		},
	}
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// success
	r, err := c.PodResource(ctx, podname)
	assert.NoError(t, err)
	assert.Len(t, r.CPUPercents, 1)
	assert.Len(t, r.MemoryPercents, 1)
	assert.Len(t, r.StoragePercents, 1)
	assert.False(t, r.Verifications[nodename])
	assert.NotEmpty(t, r.Details[nodename])
}

func TestNodeResource(t *testing.T) {
	c := NewTestCluster()
	ctx := context.Background()
	nodename := "testnode"
	store := &storemocks.Store{}
	c.store = store
	lock := &lockmocks.DistributedLock{}
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	lock.On("Lock", mock.Anything).Return(nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	node := &types.Node{
		Name:           nodename,
		CPU:            types.CPUMap{"0": 0, "1": 10},
		MemCap:         2,
		InitCPU:        types.CPUMap{"0": 100, "1": 100},
		InitMemCap:     6,
		NUMAMemory:     types.NUMAMemory{"0": 1, "1": 1},
		InitNUMAMemory: types.NUMAMemory{"0": 3, "1": 3},
	}
	engine := &enginemocks.API{}
	engine.On("ResourceValidate", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		fmt.Errorf("%s", "not validate"),
	)
	node.Engine = engine
	// failed by GetNode
	store.On("GetNode", ctx, nodename).Return(nil, types.ErrNoETCD).Once()
	_, err := c.NodeResource(ctx, nodename, false)
	assert.Error(t, err)
	store.On("GetNode", mock.Anything, nodename).Return(node, nil)
	// failed by list node containers
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err = c.NodeResource(ctx, nodename, false)
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
	store.On("ListNodeContainers", mock.Anything, mock.Anything, mock.Anything).Return(containers, nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	// success but container inspect failed
	nr, err := c.NodeResource(ctx, nodename, true)
	assert.NoError(t, err)
	assert.Equal(t, nr.Name, nodename)
	assert.NotEmpty(t, nr.Details)
	assert.False(t, nr.Verification)
	details := strings.Join(nr.Details, ",")
	assert.Contains(t, details, "inspect failed")
}

func TestAllocResource(t *testing.T) {
	c := NewTestCluster()
	scheduler.InitSchedulerV1(c.scheduler)
	ctx := context.Background()
	podname := "testpod"
	opts := &types.DeployOptions{
		Podname: podname,
	}
	config := types.Config{
		LockTimeout: time.Duration(time.Second * 3),
	}
	c.config = config
	n1 := "n2"
	n2 := "n2"
	nodeMap := map[string]*types.Node{
		n1: &types.Node{
			Name:      n1,
			Available: false,
			Labels:    map[string]string{"test": "1"},
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
		n2: &types.Node{
			Name:      n2,
			Available: true,
			CPU:       types.CPUMap{"0": 100},
			MemCap:    100,
		},
	}

	store := c.store.(*storemocks.Store)
	defer store.AssertExpectations(t)

	// Defines for below.
	opts.Nodename = n2

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

	testAllocFailedAsMakeDeployStatusError(t, c, opts, nodeMap)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	testAllocFailedAsInsufficientMemory(t, c, opts, nodeMap)

	sched := c.scheduler.(*schedulermocks.Scheduler)
	defer sched.AssertExpectations(t)

	total := 3
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nodesInfo, total, nil)
	sched.On("SelectStorageNodes", mock.Anything, mock.Anything).Return(nodesInfo, total, nil)

	testAllocFailedAsWrongDeployMethod(t, c, opts, nodeMap)
	testAllocFailedAsCommonDivisionError(t, c, opts, nodeMap)

	// Mocks for all.
	opts.DeployStrategy = strategy.Fill
	oldFillFunc := strategy.Plans[strategy.Fill]
	strategy.Plans[strategy.Fill] = func(sis []types.StrategyInfo, need, _, limit int, resourceType types.ResourceType) (map[string]*types.DeployInfo, error) {
		dis := make(map[string]*types.DeployInfo)
		for _, si := range sis {
			dis[si.Nodename] = &types.DeployInfo{Deploy: 3}
		}
		return dis, nil
	}
	defer func() {
		strategy.Plans[strategy.Fill] = oldFillFunc
	}()

	// success
	opts.ResourceRequests = []types.ResourceRequest{
		resources.CPUMemResourceRequest{CPUQuota: 1, Memory: 1},
		resources.StorageResourceRequest{Quota: 1},
	}
	_, _, err := c.doAllocResource(ctx, nodeMap, opts)
	assert.NoError(t, err)
}

func testAllocFailedAsMakeDeployStatusError(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {
	store := c.store.(*storemocks.Store)
	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNoETCD).Once()
	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
}

func testAllocFailedAsInsufficientMemory(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {
	sched := c.scheduler.(*schedulermocks.Scheduler)
	sched.On("SelectMemoryNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil, 0, types.ErrInsufficientMEM).Once()
	opts.ResourceRequests = []types.ResourceRequest{
		resources.CPUMemResourceRequest{CPUQuota: 1, Memory: 1},
	}
	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
}

func testAllocFailedAsWrongDeployMethod(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {

	opts.DeployStrategy = "invalid"
	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
	opts.DeployStrategy = "AUTO"
}

func testAllocFailedAsCommonDivisionError(t *testing.T, c *Calcium, opts *types.DeployOptions, nodeMap map[string]*types.Node) {
	opts.DeployStrategy = strategy.Auto
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(_ []types.StrategyInfo, need, total, _ int, resourceType types.ResourceType) (map[string]*types.DeployInfo, error) {
		return nil, types.ErrInsufficientRes
	}
	defer func() {
		strategy.Plans[strategy.Auto] = old
	}()

	_, _, err := c.doAllocResource(context.Background(), nodeMap, opts)
	assert.Error(t, err)
}
