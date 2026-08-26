package calcium

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/cockroachdb/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcemocks "github.com/projecteru2/core/resource/mocks"
	plugintypes "github.com/projecteru2/core/resource/plugins/types"
	resourcetypes "github.com/projecteru2/core/resource/types"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestCreateWorkloadValidating(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	opts := &types.DeployOptions{
		Name:    "deployname",
		Podname: "somepod",
		Image:   "image:todeploy",
		Count:   1,
		Entrypoint: &types.Entrypoint{
			Name: "some-nice-entrypoint",
		},
		NodeFilter: &types.NodeFilter{},
	}
	opts.Name = ""
	_, err := c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Name = "deployname"

	opts.Podname = ""
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Podname = "somepod"

	opts.Image = ""
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Image = "image:todeploy"

	opts.Count = 0
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Count = 1

	opts.Entrypoint.Name = "bad_entry_name"
	_, err = c.CreateWorkload(ctx, opts)
	assert.Error(t, err)
	opts.Entrypoint.Name = "some-nice-entrypoint"
}

func TestCreateWorkloadTxn(t *testing.T) {
	for _, ignorePull := range []bool{false, true} {
		t.Run(fmt.Sprintf("ignorePull=%t", ignorePull), func(t *testing.T) {
			c, nodes := newCreateWorkloadCluster(t, nil, nil)
			ctx := t.Context()
			opts := &types.DeployOptions{
				Name:           "zc:name",
				Count:          2,
				DeployStrategy: strategy.Auto,
				Podname:        "p1",
				Resources:      resourcetypes.Resources{},
				Image:          "zc:test",
				Entrypoint: &types.Entrypoint{
					Name: "good-entrypoint",
				},
				NodeFilter: &types.NodeFilter{},
				IgnorePull: ignorePull,
			}

			store := c.store.(*storemocks.Store)
			rmgr := c.rmgr.(*resourcemocks.Manager)
			mwal := &walmocks.WAL{}
			c.wal = mwal
			var walCommitted atomic.Bool
			commit := wal.Commit(func() error {
				walCommitted.Store(true)
				return nil
			})
			mwal.On("Log", mock.Anything, mock.Anything).Return(commit, nil)
			node1, node2 := nodes[0], nodes[1]

			rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
				nil, 0, types.ErrMockError,
			).Once()
			ch, err := c.CreateWorkload(ctx, opts)
			assert.Nil(t, err)
			cnt := 0
			for m := range ch {
				cnt++
				assert.Error(t, m.Error, "key is empty")
			}
			assert.EqualValues(t, 1, cnt)
			assert.True(t, walCommitted.Load())
			walCommitted.Store(false)
			rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
				map[string]*plugintypes.NodeDeployCapacity{
					node1.Name: {
						Capacity: 10,
						Usage:    0.5,
						Rate:     0.05,
						Weight:   100,
					},
					node2.Name: {
						Capacity: 10,
						Usage:    0.5,
						Rate:     0.05,
						Weight:   100,
					},
				}, 20, nil,
			)

			store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "GetDeployStatus")).Once()
			ch, err = c.CreateWorkload(ctx, opts)
			assert.Nil(t, err)
			cnt = 0
			for m := range ch {
				cnt++
				assert.ErrorIs(t, m.Error, context.DeadlineExceeded)
				assert.Error(t, m.Error, "GetDeployStatus")
			}
			assert.EqualValues(t, 1, cnt)
			assert.True(t, walCommitted.Load())
			walCommitted.Store(false)
			store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{}, nil)

			rmgr.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
				nil, nil, types.ErrMockError,
			).Once()
			ch, err = c.CreateWorkload(ctx, opts)
			assert.Nil(t, err)
			cnt = 0
			for m := range ch {
				cnt++
				assert.Error(t, m.Error, "DeadlineExceeded")
			}
			assert.EqualValues(t, 1, cnt)
			assert.True(t, walCommitted.Load())
			walCommitted.Store(false)
			rmgr.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
				[]resourcetypes.Resources{{}, {}},
				[]resourcetypes.Resources{
					{node1.Name: {}},
					{node2.Name: {}},
				},
				nil,
			)
			rmgr.On("RollbackAlloc", mock.Anything, mock.Anything, mock.Anything).Return(nil)
			store.On("GetNode",
				mock.AnythingOfType("*context.timerCtx"),
				mock.AnythingOfType("string"),
			).Return(
				func(_ context.Context, name string) (node *types.Node) {
					node = node1
					if name == "n2" {
						node = node2
					}
					return node
				}, nil,
			)
			engine := node1.Engine.(*enginemocks.API)
			store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

			if !ignorePull {
				engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImageLocalDigest")).Twice()
				engine.On("ImagePull", mock.Anything, mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "ImagePull")).Twice()
				ch, err = c.CreateWorkload(ctx, opts)
				assert.Nil(t, err)
				cnt = 0
				for m := range ch {
					cnt++
					assert.Error(t, m.Error, "ImagePull")
				}
				assert.EqualValues(t, 2, cnt)
				assert.True(t, walCommitted.Load())

				engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{""}, nil)
				engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", nil)
			}

			engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, errors.Wrap(context.DeadlineExceeded, "VirtualizationCreate")).Twice()
			engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(nil, types.ErrWorkloadNotExists).Twice()
			engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
			store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
			walCommitted.Store(false)
			ch, err = c.CreateWorkload(ctx, opts)
			assert.Nil(t, err)
			cnt = 0
			for m := range ch {
				cnt++
				assert.Error(t, m.Error)
				assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
				assert.Error(t, m.Error, "VirtualizationCreate")
			}
			assert.EqualValues(t, 2, cnt)
			assert.True(t, walCommitted.Load())

			engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
			engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
			engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
			store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddWorkload")).Twice()
			walCommitted.Store(false)
			ch, err = c.CreateWorkload(ctx, opts)
			assert.Nil(t, err)
			cnt = 0
			for m := range ch {
				cnt++
				assert.Error(t, m.Error)
				assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
				assert.Error(t, m.Error, "AddWorkload")
			}
			assert.EqualValues(t, 2, cnt)
			assert.True(t, walCommitted.Load())

			engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "c1"}, nil)
			engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
			engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
			store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(errors.Wrap(context.DeadlineExceeded, "AddWorkload2")).Once()
			store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(nil).Once()
			walCommitted.Store(false)
			ch, err = c.CreateWorkload(ctx, opts)
			assert.Nil(t, err)
			cnt = 0
			errCnt := 0
			for m := range ch {
				cnt++
				if m.Error != nil {
					assert.Error(t, m.Error)
					assert.True(t, errors.Is(m.Error, context.DeadlineExceeded))
					assert.Error(t, m.Error, "AddWorkload2")
					errCnt++
				}
			}
			assert.EqualValues(t, 2, cnt)
			assert.EqualValues(t, 1, errCnt)
			assert.True(t, walCommitted.Load())
			store.AssertExpectations(t)
			engine.AssertExpectations(t)
		})
	}
}

func TestCreateWorkloadRollsBackAllocatedResourcesAfterProcessingFailure(t *testing.T) {
	for _, tc := range []struct {
		name                string
		rollbackErr         error
		deleteProcessingErr error
		resourceCommitted   bool
		processingCommitted bool
	}{
		{name: "rollback succeeds", resourceCommitted: true, processingCommitted: true},
		{name: "rollback fails", rollbackErr: types.ErrMockError, processingCommitted: true},
		{name: "processing cleanup fails", deleteProcessingErr: types.ErrMockError, resourceCommitted: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c, nodes := newCreateWorkloadCluster(t, types.ErrMockError, tc.deleteProcessingErr)
			ctx := t.Context()
			opts := &types.DeployOptions{
				Name:           "zc:name",
				Count:          1,
				DeployStrategy: strategy.Auto,
				Podname:        "p1",
				Resources:      resourcetypes.Resources{},
				Image:          "zc:test",
				Entrypoint:     &types.Entrypoint{Name: "good-entrypoint"},
				NodeFilter:     &types.NodeFilter{},
			}

			store := c.store.(*storemocks.Store)
			rmgr := c.rmgr.(*resourcemocks.Manager)
			mwal := &walmocks.WAL{}
			c.wal = mwal
			var resourceCommitted atomic.Bool
			var processingCommitted atomic.Bool
			mwal.On("Log", eventWorkloadResourceAllocated, mock.Anything).Return(wal.Commit(func() error {
				resourceCommitted.Store(true)
				return nil
			}), nil).Once()
			mwal.On("Log", eventProcessingCreated, mock.Anything).Return(wal.Commit(func() error {
				processingCommitted.Store(true)
				return nil
			}), nil).Once()

			node := nodes[0]
			rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
				map[string]*plugintypes.NodeDeployCapacity{
					node.Name: {Capacity: 1, Weight: 1},
				},
				1,
				nil,
			).Once()
			store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{}, nil).Once()
			rmgr.On("Alloc", mock.Anything, node.Name, 1, mock.Anything).Return(
				[]resourcetypes.Resources{{}},
				[]resourcetypes.Resources{{}},
				nil,
			).Once()
			rmgr.On("RollbackAlloc", mock.Anything, node.Name, []resourcetypes.Resources{{}}).Return(tc.rollbackErr).Once()

			ch, err := c.CreateWorkload(ctx, opts)
			require.NoError(t, err)
			messages := []*types.CreateWorkloadMessage{}
			for message := range ch {
				messages = append(messages, message)
			}
			require.Len(t, messages, 1)
			assert.ErrorIs(t, messages[0].Error, types.ErrMockError)
			assert.Equal(t, tc.resourceCommitted, resourceCommitted.Load())
			assert.Equal(t, tc.processingCommitted, processingCommitted.Load())
			mwal.AssertExpectations(t)
		})
	}
}

func TestDoDeployWorkloadsOnNodeErrorPerWorkload(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	engine := &enginemocks.API{}
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}, Engine: engine}

	store := c.store.(*storemocks.Store)
	store.On("GetNode", mock.Anything, mock.Anything).Return(node, nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(ctx, nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(nil, types.ErrWorkloadNotExists)

	const deploy = 4
	opts := &types.DeployOptions{
		Name:       "app",
		Podname:    "pod",
		IgnorePull: true,
		Entrypoint: &types.Entrypoint{Name: "entry"},
	}
	ch := make(chan *types.CreateWorkloadMessage, deploy)
	params := make([]resourcetypes.Resources, deploy)

	indices, err := c.doDeployWorkloadsOnNode(ctx, ch, node.Name, opts, deploy, params, params, 0)
	close(ch)

	assert.Error(t, err)
	assert.Len(t, indices, deploy)
	for m := range ch {
		assert.Error(t, m.Error)
	}
}

func TestDoDeployOneWorkloadJournalsTheNameBeforeTheEngineCreate(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	var logged *types.Workload
	engineCalled, loggedAfterEngine := false, false
	mwal := &walmocks.WAL{}
	mwal.On("Log", mock.Anything, mock.Anything).Return(func(_ string, raw any) (wal.Commit, error) {
		logged, _ = raw.(*types.Workload)
		loggedAfterEngine = engineCalled
		return func() error { return nil }, nil
	})
	c.wal = mwal

	engine := &enginemocks.API{}
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).
		Run(func(mock.Arguments) { engineCalled = true }).
		Return(nil, types.ErrMockError)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(nil, types.ErrWorkloadNotExists)
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}, Engine: engine}

	opts := &types.DeployOptions{Name: "app", Podname: "pod", Entrypoint: &types.Entrypoint{Name: "entry"}}
	createOpts := &enginetypes.VirtualizationCreateOptions{Name: "app_entry_abcdef"}

	assert.Error(t, c.doDeployOneWorkload(ctx, node, opts, &types.CreateWorkloadMessage{}, createOpts, false))
	require.NotNil(t, logged, "the create was not journalled")
	assert.False(t, loggedAfterEngine)
	assert.Equal(t, createOpts.Name, logged.Name)
	assert.Equal(t, node.Name, logged.Nodename)
	assert.Empty(t, logged.ID)
}

func TestDoDeployOneWorkloadRollbackRemovesTheContainerTheEngineKept(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()

	name := "app_entry_abcdef"
	engine := &enginemocks.API{}
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
	engine.On("VirtualizationInspect", mock.Anything, name).Return(&enginetypes.VirtualizationInfo{ID: "wrkid"}, nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, "wrkid", true, true).Return(nil).Once()
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}, Engine: engine}

	opts := &types.DeployOptions{Name: "app", Podname: "pod", Entrypoint: &types.Entrypoint{Name: "entry"}}
	createOpts := &enginetypes.VirtualizationCreateOptions{Name: name}

	assert.Error(t, c.doDeployOneWorkload(ctx, node, opts, &types.CreateWorkloadMessage{}, createOpts, false))
	engine.AssertExpectations(t)
}

func TestDoDeployOneWorkloadKeepsJournalWhenRollbackFails(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	var committed atomic.Bool
	mwal := &walmocks.WAL{}
	mwal.On("Log", eventWorkloadCreated, mock.Anything).Return(wal.Commit(func() error {
		committed.Store(true)
		return nil
	}), nil).Once()
	c.wal = mwal

	store := c.store.(*storemocks.Store)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(types.ErrMockError).Once()
	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil).Once()
	engine := &enginemocks.API{}
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "wrkid"}, nil).Once()
	engine.On("VirtualizationRemove", mock.Anything, "wrkid", true, true).Return(types.ErrMockError).Once()
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}, Engine: engine}

	opts := &types.DeployOptions{Name: "app", Podname: "pod", Entrypoint: &types.Entrypoint{Name: "entry"}}
	createOpts := &enginetypes.VirtualizationCreateOptions{Name: "app_entry_abcdef"}

	assert.Error(t, c.doDeployOneWorkload(ctx, node, opts, &types.CreateWorkloadMessage{}, createOpts, false))
	assert.False(t, committed.Load())
	mwal.AssertExpectations(t)
	store.AssertExpectations(t)
	engine.AssertExpectations(t)
}

func TestDoMakeWorkloadOptionsEnvIsolation(t *testing.T) {
	c := NewTestCluster()
	ctx := t.Context()
	node := &types.Node{NodeMeta: types.NodeMeta{Name: "n1"}}
	opts := &types.DeployOptions{
		Name:       "app",
		Podname:    "pod",
		Entrypoint: &types.Entrypoint{Name: "entry"},
		Env:        append(make([]string, 0, 8), "A=1", "B=2"),
	}

	const n = 4
	got := make([]*enginetypes.VirtualizationCreateOptions, n)
	wg := sync.WaitGroup{}
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			got[i] = c.doMakeWorkloadOptions(ctx, i, &types.CreateWorkloadMessage{}, opts, node)
		}()
	}
	wg.Wait()

	for i := range n {
		assert.Contains(t, got[i].Env, fmt.Sprintf("ERU_WORKLOAD_SEQ=%d", i))
		assert.Contains(t, got[i].Env, "A=1")
	}
	assert.Equal(t, []string{"A=1", "B=2"}, opts.Env)
}

func newCreateWorkloadCluster(t *testing.T, createProcessingErr, deleteProcessingErr error) (*Calcium, []*types.Node) {
	c := NewTestCluster()

	engine := &enginemocks.API{}
	node1 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "n1",
		},
		Engine: engine,
	}
	node2 := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "n2",
		},
		Engine: engine,
	}
	nodes := []*types.Node{node1, node2}

	store := c.store.(*storemocks.Store)
	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(createProcessingErr)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(deleteProcessingErr)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(t.Context(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)

	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("GetNode", mock.Anything, mock.Anything).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return node
		}, nil,
	)

	store.On("RemoveWorkload", mock.Anything, mock.Anything).Return(nil)

	return c, nodes
}
