package calcium

import (
	"context"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

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

func TestRunAndWaitFailedThenWALCommitted(t *testing.T) {
	assert := assert.New(t)
	c, _ := newCreateWorkloadCluster(t, nil, nil)

	rmgr := &resourcemocks.Manager{}
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil, nil, nil)
	rmgr.On("GetNodesDeployCapacity", mock.Anything, mock.Anything, mock.Anything).Return(
		nil, 0, types.ErrMockError,
	)
	c.rmgr = rmgr

	mwal := c.wal.(*walmocks.WAL)
	defer mwal.AssertNotCalled(t, "Log")
	mwal.On("Log", mock.Anything, mock.Anything).Return(nil, nil)

	opts := lambdaOptions()

	_, ch, err := c.RunAndWait(t.Context(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch)
	ms := drainAttachMessages(ch)
	m := ms[0]
	assert.Equal(m.WorkloadID, "")
	assert.True(strings.HasPrefix(string(m.Data), "Create workload failed"))

	assert.Equal(m.StdStreamType, types.EruError)
}

func TestLambdaWithWorkloadIDReturned(t *testing.T) {
	assert := assert.New(t)
	c, nodes := newLambdaCluster(t)
	engine := nodes[0].Engine.(*enginemocks.API)
	store := c.store.(*storemocks.Store)
	workload := &types.Workload{ID: "workloadfortonictest", Engine: engine}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)

	opts := lambdaOptions()

	stdout, stderr := stdPipes()
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(stdout, stderr, nil)
	engine.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationWaitResult{Code: 0}, nil)

	ids, ch, err := c.RunAndWait(t.Context(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch)
	assert.Equal(len(ids), 2)
	assert.Equal(ids[0], "workloadfortonictest")

	ms := drainAttachMessages(ch)
	assert.Equal(len(ms), 6)
	assert.True(strings.HasPrefix(string(ms[5].Data), exitDataPrefix))
	assert.Equal(ms[5].StdStreamType, types.Stdout)
}

func TestLambdaWithError(t *testing.T) {
	assert := assert.New(t)
	c, nodes := newLambdaCluster(t)
	engine := nodes[0].Engine.(*enginemocks.API)

	workload := &types.Workload{ID: "workloadfortonictest", Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)

	opts := lambdaOptions()

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error")).Twice()
	_, ch0, err := c.RunAndWait(t.Context(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch0)
	m0 := <-ch0
	assert.Equal(m0.WorkloadID, "workloadfortonictest")
	assert.True(strings.HasPrefix(string(m0.Data), "Get workload"))
	assert.Equal(m0.StdStreamType, types.EruError)

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)

	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, nil, fmt.Errorf("error")).Twice()
	_, ch1, err := c.RunAndWait(t.Context(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch1)
	m1 := <-ch1
	assert.Equal(m1.WorkloadID, "workloadfortonictest")
	assert.True(strings.HasPrefix(string(m1.Data), "Fetch log for workload"))
	assert.Equal(m1.StdStreamType, types.EruError)

	stdout, stderr := stdPipes()
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(stdout, stderr, nil)

	engine.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error"))
	ids, ch2, err := c.RunAndWait(t.Context(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch2)
	assert.Equal(ids[0], "workloadfortonictest")
	assert.Equal(ids[1], "workloadfortonictest")

	ms := drainAttachMessages(ch2)
	assert.Equal(len(ms), 6)
	assert.Equal(ms[5].WorkloadID, "workloadfortonictest")
	assert.True(strings.HasPrefix(string(ms[5].Data), "Wait workload"))
	assert.Equal(ms[5].StdStreamType, types.EruError)
}

func TestLambdaWithStdinOpensNoFollowStream(t *testing.T) {
	assert := assert.New(t)
	c, nodes := newLambdaCluster(t)
	engine := nodes[0].Engine.(*enginemocks.API)

	workload := &types.Workload{ID: "workloadfortonictest", Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	engine.On("VirtualizationAttach", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil, nil, nil, types.ErrEngineNotImplemented)

	opts := lambdaOptions()
	opts.Count = 1
	opts.OpenStdin = true

	_, ch, err := c.RunAndWait(t.Context(), opts, make(chan []byte))
	assert.NoError(err)
	ms := drainAttachMessages(ch)
	assert.Len(ms, 1)
	assert.True(strings.HasPrefix(string(ms[0].Data), "Attach to workload"))
	assert.Equal(ms[0].StdStreamType, types.EruError)
	engine.AssertNotCalled(t, "VirtualizationLogs", mock.Anything, mock.Anything)
}

func TestLambdaKeepsTheJournalEntryWhenTheRemoveFails(t *testing.T) {
	assert := assert.New(t)
	c, nodes := newLambdaCluster(t)
	engine := nodes[0].Engine.(*enginemocks.API)

	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(&types.Workload{ID: "workloadfortonictest", Engine: engine}, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, nil, types.ErrMockError)

	committed := &atomic.Int64{}
	c.wal = lambdaWAL(committed)

	ids, ch, err := c.RunAndWait(t.Context(), lambdaOptions(), make(chan []byte))
	assert.NoError(err)
	assert.Len(drainAttachMessages(ch), len(ids))
	assert.Zero(committed.Load())
}

func TestLambdaCommitsTheJournalEntryAfterTheRemove(t *testing.T) {
	assert := assert.New(t)
	c, nodes := newLambdaCluster(t)
	engine := nodes[0].Engine.(*enginemocks.API)

	workload := &types.Workload{ID: "workloadfortonictest", Nodename: "n1", Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, nil, types.ErrMockError)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, nil,
	)

	committed := &atomic.Int64{}
	c.wal = lambdaWAL(committed)

	ids, ch, err := c.RunAndWait(t.Context(), lambdaOptions(), make(chan []byte))
	assert.NoError(err)
	assert.Len(drainAttachMessages(ch), len(ids))
	assert.Equal(int64(len(ids)), committed.Load())
}

func stdPipes() (stdout, stderr io.ReadCloser) {
	r1, w1 := io.Pipe()
	go func() {
		w1.Write([]byte("stdout line1\n"))
		w1.Write([]byte("stdout line2\n"))
		w1.Close()
	}()
	r2, w2 := io.Pipe()
	go func() {
		w2.Write([]byte("stderr line1\n"))
		w2.Write([]byte("stderr line2\n"))
		w2.Close()
	}()
	return io.NopCloser(r1), io.NopCloser(r2)
}

func drainAttachMessages(ch <-chan *types.AttachWorkloadMessage) []*types.AttachWorkloadMessage {
	ms := []*types.AttachWorkloadMessage{}
	for m := range ch {
		ms = append(ms, m)
	}
	return ms
}

func lambdaOptions() *types.DeployOptions {
	return &types.DeployOptions{
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
	}
}

func lambdaWAL(committed *atomic.Int64) *walmocks.WAL {
	mwal := &walmocks.WAL{}
	mwal.On("Log", eventCreateLambda, mock.Anything).Return(wal.Commit(func() error {
		committed.Add(1)
		return nil
	}), nil)
	mwal.On("Log", mock.Anything, mock.Anything).Return(wal.Commit(func() error { return nil }), nil)
	return mwal
}

func newLambdaCluster(t *testing.T) (*Calcium, []*types.Node) {
	c, nodes := newCreateWorkloadCluster(t, nil, nil)
	node1, node2 := nodes[0], nodes[1]

	store := c.store.(*storemocks.Store)
	rmgr := c.rmgr.(*resourcemocks.Manager)
	rmgr.On("SetNodeResourceUsage", mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{}, resourcetypes.Resources{}, nil,
	)
	rmgr.On("GetNodeResourceInfo", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		resourcetypes.Resources{},
		resourcetypes.Resources{},
		[]string{},
		nil,
	)
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
		},
		20, nil,
	)
	rmgr.On("Alloc", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
		[]resourcetypes.Resources{{}, {}},
		[]resourcetypes.Resources{
			{node1.Name: {}},
			{node2.Name: {}},
		},
		nil,
	)
	store.On("GetDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(map[string]int{}, nil)
	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("GetNode",
		mock.Anything,
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

	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{""}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", nil)

	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "workloadfortonictest"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return c, nodes
}
