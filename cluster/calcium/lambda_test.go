package calcium

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/wal"
	walmocks "github.com/projecteru2/core/wal/mocks"
)

func TestRunAndWaitFailedThenWALCommitted(t *testing.T) {
	c, _ := newCreateWorkloadCluster(t)
	c.wal = &WAL{WAL: &walmocks.WAL{}}

	mwal := c.wal.WAL.(*walmocks.WAL)
	defer mwal.AssertExpectations(t)
	var walCommitted bool
	commit := wal.Commit(func() error {
		walCommitted = true
		return nil
	})
	mwal.On("Log", eventCreateLambda, mock.Anything).Return(commit, nil).Once()

	opts := &types.DeployOptions{
		Name:           "zc:name",
		Count:          2,
		DeployStrategy: strategy.Auto,
		Podname:        "p1",
		ResourceOpts:   types.ResourceOptions{CPUQuotaLimit: 1},
		Image:          "zc:test",
		Entrypoint: &types.Entrypoint{
			Name: "good-entrypoint",
		},
	}

	mstore := c.store.(*storemocks.Store)
	mstore.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("err")).Once()

	ch, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	require.NoError(t, err)
	require.NotNil(t, ch)
	require.False(t, walCommitted)
	require.Nil(t, <-ch) // recv nil due to the ch will be closed.

	lambdaID, exists := opts.Labels[labelLambdaID]
	require.True(t, exists)
	require.True(t, len(lambdaID) > 1)
	require.True(t, walCommitted)
}

func TestLambdaWithWorkloadIDReturned(t *testing.T) {
	c, _ := newLambdaCluster(t)

	opts := &types.DeployOptions{
		Name:           "zc:name",
		Count:          2,
		DeployStrategy: strategy.Auto,
		Podname:        "p1",
		ResourceOpts:   types.ResourceOptions{CPUQuotaLimit: 1},
		Image:          "zc:test",
		Entrypoint: &types.Entrypoint{
			Name: "good-entrypoint",
		},
	}

	ch, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	assert.NoError(t, err)
	assert.NotNil(t, ch)

	m := <-ch
	require.Equal(t, m.WorkloadID, "workloadfortonictest")
}

func newLambdaCluster(t *testing.T) (*Calcium, []*types.Node) {
	c, nodes := newCreateWorkloadCluster(t)

	store := &storemocks.Store{}
	sche := &schedulermocks.Scheduler{}
	scheduler.InitSchedulerV1(sche)
	c.store = store
	c.scheduler = sche

	node1, node2 := nodes[0], nodes[1]

	store.On("SaveProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateProcessing", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("DeleteProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	lock := &lockmocks.DistributedLock{}
	lock.On("Lock", mock.Anything).Return(context.Background(), nil)
	lock.On("Unlock", mock.Anything).Return(nil)
	store.On("CreateLock", mock.Anything, mock.Anything).Return(lock, nil)
	store.On("GetNodesByPod", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nodes, nil)
	store.On("GetNode",
		mock.AnythingOfType("*context.emptyCtx"),
		mock.AnythingOfType("string"),
	).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)
	sche.On("SelectStorageNodes", mock.AnythingOfType("[]resourcetypes.ScheduleInfo"), mock.AnythingOfType("int64")).Return(func(scheduleInfos []resourcetypes.ScheduleInfo, _ int64) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, len(nodes), nil)
	sche.On("SelectStorageNodes", mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("int64")).Return(func(scheduleInfos []resourcetypes.ScheduleInfo, _ int64) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, len(nodes), nil)
	sche.On("SelectVolumeNodes", mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("types.VolumeBindings")).Return(func(scheduleInfos []resourcetypes.ScheduleInfo, _ types.VolumeBindings) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, nil, len(nodes), nil)
	sche.On("SelectMemoryNodes", mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("float64"), mock.AnythingOfType("int64")).Return(
		func(scheduleInfos []resourcetypes.ScheduleInfo, _ float64, _ int64) []resourcetypes.ScheduleInfo {
			for i := range scheduleInfos {
				scheduleInfos[i].Capacity = 1
			}
			return scheduleInfos
		}, len(nodes), nil)

	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(sis []strategy.Info, need, total, _ int) (map[string]int, error) {
		deployInfos := make(map[string]int)
		for _, si := range sis {
			deployInfos[si.Nodename] = 1
		}
		return deployInfos, nil
	}
	defer func() {
		strategy.Plans[strategy.Auto] = old
	}()

	store.On("UpdateNodes", mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("UpdateNodes", mock.Anything, mock.Anything).Return(nil)
	store.On("GetNode",
		mock.AnythingOfType("*context.timerCtx"),
		mock.AnythingOfType("string"),
	).Return(
		func(_ context.Context, name string) (node *types.Node) {
			node = node1
			if name == "n2" {
				node = node2
			}
			return
		}, nil)
	engine := node1.Engine.(*enginemocks.API)

	// doDeployOneWorkload fails: VirtualizationCreate
	engine.On("ImageLocalDigests", mock.Anything, mock.Anything).Return([]string{""}, nil)
	engine.On("ImageRemoteDigest", mock.Anything, mock.Anything).Return("", nil)

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
	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(ioutil.NopCloser(r1), ioutil.NopCloser(r2), nil)
	engine.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationWaitResult{Code: 0})

	// doCreateAndStartWorkload fails: AddWorkload
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "workloadfortonictest"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything).Return(nil)

	workload := &types.Workload{ID: "workloadfortonictest", Engine: engine}
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	return c, nodes
}
