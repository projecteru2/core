package calcium

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/projecteru2/core/cluster"
	enginemocks "github.com/projecteru2/core/engine/mocks"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/types"
)

func TestControlStartResume(t *testing.T) {
	tests := []struct {
		name       string
		controlTyp string
		engineOp   string
		newHook    func() *types.Hook
	}{
		{
			name:       "Start",
			controlTyp: cluster.WorkloadStart,
			engineOp:   "VirtualizationStart",
			newHook:    func() *types.Hook { return &types.Hook{AfterStart: []string{"cmd1", "cmd2"}} },
		},
		{
			name:       "Resume",
			controlTyp: cluster.WorkloadResume,
			engineOp:   "VirtualizationResume",
			newHook:    func() *types.Hook { return &types.Hook{AfterResume: []string{"cmd1", "cmd2"}} },
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c, ctx, store := newControlTestCluster(t)
			store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
			ch, err := c.ControlWorkload(ctx, []string{"id1"}, "", true)
			assert.NoError(t, err)
			for r := range ch {
				assert.Error(t, r.Error)
			}
			workload := &types.Workload{
				ID:         "id1",
				Privileged: true,
			}
			engine := &enginemocks.API{}
			workload.Engine = engine
			store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, "", true)
			assert.NoError(t, err)
			for r := range ch {
				assert.Error(t, r.Error)
			}
			engine.On(tt.engineOp, mock.Anything, mock.Anything).Return(types.ErrNilEngine).Once()
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, tt.controlTyp, false)
			assert.NoError(t, err)
			for r := range ch {
				assert.Error(t, r.Error)
			}
			engine.On(tt.engineOp, mock.Anything, mock.Anything).Return(nil)
			hook := tt.newHook()
			workload.Hook = hook
			workload.Hook.Force = false
			engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return("", nil, nil, nil, types.ErrNilEngine).Times(3)
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, tt.controlTyp, false)
			assert.NoError(t, err)
			for r := range ch {
				assert.NoError(t, r.Error)
			}
			workload.Hook.Force = true
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, tt.controlTyp, false)
			assert.NoError(t, err)
			for r := range ch {
				assert.Error(t, r.Error)
				assert.Equal(t, r.WorkloadID, "id1")
			}
			data := io.NopCloser(bytes.NewBufferString("output"))
			engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return("eid", data, nil, nil, nil).Times(4)
			engine.On("ExecExitCode", mock.Anything, mock.Anything, mock.Anything).Return(-1, types.ErrNilEngine).Once()
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, tt.controlTyp, false)
			assert.NoError(t, err)
			for r := range ch {
				assert.Error(t, r.Error)
			}
			engine.On("ExecExitCode", mock.Anything, mock.Anything, mock.Anything).Return(-1, nil).Once()
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, tt.controlTyp, false)
			assert.NoError(t, err)
			for r := range ch {
				assert.Error(t, r.Error)
			}
			engine.On("ExecExitCode", mock.Anything, mock.Anything, mock.Anything).Return(0, nil)
			ch, err = c.ControlWorkload(ctx, []string{"id1"}, tt.controlTyp, false)
			assert.NoError(t, err)
			for r := range ch {
				assert.NoError(t, r.Error)
			}
		})
	}
}

func TestControlStop(t *testing.T) {
	c, ctx, store := newControlTestCluster(t)
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	hook := &types.Hook{
		BeforeStop: []string{"cmd1"},
	}
	workload.Hook = hook
	workload.Hook.Force = true
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return("", nil, nil, nil, types.ErrNilEngine)
	ch, err := c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadStop, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	workload.Hook.Force = false
	engine.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNilEngine).Once()
	ch, err = c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadStop, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err = c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadStop, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
}

func TestControlRestart(t *testing.T) {
	c, ctx, store := newControlTestCluster(t)
	engine := &enginemocks.API{}
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
		Engine:     engine,
	}
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	hook := &types.Hook{
		BeforeStop: []string{"cmd1"},
	}
	workload.Hook = hook
	workload.Hook.Force = true
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return("", nil, nil, nil, types.ErrNilEngine)
	ch, err := c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadRestart, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	workload.Hook = nil
	engine.On("VirtualizationStop", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	ch, err = c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadRestart, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
}

func TestControlSuspend(t *testing.T) {
	c, ctx, store := newControlTestCluster(t)
	workload := &types.Workload{
		ID:         "id1",
		Privileged: true,
	}
	engine := &enginemocks.API{}
	workload.Engine = engine
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	hook := &types.Hook{
		BeforeSuspend: []string{"cmd1"},
	}
	workload.Hook = hook
	workload.Hook.Force = true
	engine.On("Execute", mock.Anything, mock.Anything, mock.Anything).Return("", nil, nil, nil, types.ErrNilEngine)
	ch, err := c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadSuspend, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	workload.Hook.Force = false
	ch, err = c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadSuspend, false)
	engine.On("VirtualizationSuspend", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrNilEngine).Once()
	assert.NoError(t, err)
	for r := range ch {
		assert.Error(t, r.Error)
	}
	engine.On("VirtualizationSuspend", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	ch, err = c.ControlWorkload(ctx, []string{"id1"}, cluster.WorkloadSuspend, false)
	assert.NoError(t, err)
	for r := range ch {
		assert.NoError(t, r.Error)
	}
}

func newControlTestCluster(t *testing.T) (*Calcium, context.Context, *storemocks.Store) {
	c := NewTestCluster()
	ctx := t.Context()
	store := c.store.(*storemocks.Store)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	return c, ctx, store
}
