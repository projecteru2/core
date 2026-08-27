package calcium

import (
	"context"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestRealloc(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	rmgr.On("GetNodeMetrics", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return([]*plugintypes.Metrics{}, nil)
	c.config.Scheduler.ShareBase = 100

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)

	engine := &enginemocks.API{}
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)

	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "node1",
			Endpoint: "http://1.1.1.1:1",
		},
		Engine: engine,
	}

	newC1 := func(context.Context, []string) []*types.Workload {
		return []*types.Workload{
			{
				ID:        "c1",
				Podname:   "p1",
				Engine:    engine,
				Resources: resourcetypes.Resources{},
				Nodename:  "node1",
			},
		}
	}

	store.On("GetWorkload", mock.Anything, "c1").Return(newC1(t.Context(), nil)[0], nil)
	opts := &types.ReallocOptions{
		ID:        "c1",
		Resources: resourcetypes.Resources{},
	}

	store.On("GetNode", mock.Anything, "node1").Return(nil, types.ErrMockError).Once()
	err := c.ReallocResource(ctx, opts)
	assert.True(t, errors.Is(err, types.ErrMockError))
	store.AssertExpectations(t)
	store.On("GetNode", mock.Anything, "node1").Return(node1, nil)

	store.On("CreateLock", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	err = c.ReallocResource(ctx, opts)
	assert.True(t, errors.Is(err, types.ErrMockError))
	store.AssertExpectations(t)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetWorkloads", mock.Anything, []string{"c1"}).Return(newC1, nil)

	rmgr.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, nil, nil, types.ErrMockError,
	).Once()
	err = c.ReallocResource(ctx, opts)
	assert.Error(t, err)
	rmgr.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		nil,
	)
	rmgr.On("RollbackRealloc", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(types.ErrMockError).Once()
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil).Once()
	err = c.ReallocResource(ctx, opts)
	assert.True(t, errors.Is(err, types.ErrMockError))
	store.AssertExpectations(t)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil)

	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNilEngine).Once()
	c.remapped.Store("node1", &map[string]uint64{"c1": 1})
	err = c.ReallocResource(ctx, opts)
	assert.ErrorIs(t, err, types.ErrNilEngine)
	_, remembered := c.remapped.Load("node1")
	assert.False(t, remembered, "a failed realloc must forget the node so the next remap reapplies store truth")
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	err = c.ReallocResource(ctx, opts)
	assert.Nil(t, err)
}

func TestReallocJournalsRepairEntries(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	logged := []string{}
	committed := 0
	mwal := &walmocks.WAL{}
	mwal.On("Log", mock.Anything, mock.Anything).Return(func(eventyp string, _ any) (wal.Commit, error) {
		logged = append(logged, eventyp)
		return func() error { committed++; return nil }, nil
	})
	c.wal = mwal

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	engine := &enginemocks.API{}
	engine.On("VirtualizationUpdateResource", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
	workload := &types.Workload{ID: "c1", Nodename: "node1", Engine: engine, Resources: resourcetypes.Resources{}}

	store := c.store.(*storemocks.Store)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("Realloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, resourcetypes.Resources{}, nil,
	)

	opts := &types.ReallocOptions{ID: "c1", Resources: resourcetypes.Resources{}}
	assert.NoError(t, c.doReallocOnNode(ctx, node, workload, opts))
	assert.Equal(t, []string{eventWorkloadResourceAllocated, eventWorkloadReallocated}, logged)
	assert.Equal(t, 2, committed)
}

func TestReallocKeepsRepairEntriesUntilRollbackCompletes(t *testing.T) {
	for _, tc := range []struct {
		name      string
		rollback  error
		committed int
	}{
		{name: "rollback succeeds", committed: 2},
		{name: "rollback fails", rollback: types.ErrMockError},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := NewTestCluster()
			ctx := t.Context()

			committed := 0
			mwal := &walmocks.WAL{}
			mwal.On("Log", mock.Anything, mock.Anything).Return(func(_ string, _ any) (wal.Commit, error) {
				return func() error { committed++; return nil }, nil
			})
			c.wal = mwal

			engine := &enginemocks.API{}
			node := &types.Node{NodeMeta: types.NodeMeta{Name: "node1"}, Engine: engine}
			workload := &types.Workload{ID: "c1", Nodename: node.Name, Engine: engine, Resources: resourcetypes.Resources{}}

			store := c.store.(*storemocks.Store)
			store.On("UpdateWorkload", mock.Anything, workload).Return(types.ErrMockError).Once()
			store.On("UpdateWorkload", mock.Anything, mock.Anything).Return(nil).Once()
			rmgr := c.rmgr.(*resourcemocks.Manager)
			rmgr.On("Realloc", mock.Anything, node.Name, mock.Anything, mock.Anything).Return(
				resourcetypes.Resources{}, resourcetypes.Resources{}, resourcetypes.Resources{}, nil,
			).Once()
			rmgr.On("RollbackRealloc", mock.Anything, node.Name, mock.Anything).Return(tc.rollback).Once()

			opts := &types.ReallocOptions{ID: workload.ID, Resources: resourcetypes.Resources{}}
			assert.ErrorIs(t, c.doReallocOnNode(ctx, node, workload, opts), types.ErrMockError)
			assert.Equal(t, tc.committed, committed)
			mwal.AssertExpectations(t)
		})
	}
}
