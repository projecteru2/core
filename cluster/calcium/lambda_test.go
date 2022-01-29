package calcium

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"testing"

	enginemocks "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	lockmocks "github.com/projecteru2/core/lock/mocks"
	resourcetypes "github.com/projecteru2/core/resources/types"
	"github.com/projecteru2/core/scheduler"
	schedulermocks "github.com/projecteru2/core/scheduler/mocks"
	storemocks "github.com/projecteru2/core/store/mocks"
	"github.com/projecteru2/core/strategy"
	"github.com/projecteru2/core/types"
	walmocks "github.com/projecteru2/core/wal/mocks"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func TestRunAndWaitFailedThenWALCommitted(t *testing.T) {
	assert := assert.New(t)
	c, _ := newCreateWorkloadCluster(t)
	c.wal = &WAL{WAL: &walmocks.WAL{}}

	mwal := c.wal.WAL.(*walmocks.WAL)
	defer mwal.AssertNotCalled(t, "Log")
	mwal.On("Log", mock.Anything, mock.Anything).Return(nil, nil)

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
	mstore.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(fmt.Errorf("err")).Once()

	_, ch, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch)
	ms := []*types.AttachWorkloadMessage{}
	for m := range ch {
		ms = append(ms, m)
	}
	m := ms[0]
	assert.Equal(m.WorkloadID, "")
	assert.True(strings.HasPrefix(string(m.Data), "Create workload failed"))

	assert.Equal(m.StdStreamType, types.EruError)
}

func TestLambdaWithWorkloadIDReturned(t *testing.T) {
	assert := assert.New(t)
	c, nodes := newLambdaCluster(t)
	engine := nodes[0].Engine.(*enginemocks.API)

	workload := &types.Workload{ID: "workloadfortonictest", Engine: engine}
	store := c.store.(*storemocks.Store)
	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)
	store.On("GetWorkloads", mock.Anything, mock.Anything).Return([]*types.Workload{workload}, nil)

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
	engine.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationWaitResult{Code: 0}, nil)

	ids, ch, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch)
	assert.Equal(len(ids), 2)
	assert.Equal(ids[0], "workloadfortonictest")

	ms := []*types.AttachWorkloadMessage{}
	for m := range ch {
		ms = append(ms, m)
	}
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

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error")).Twice()
	_, ch0, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch0)
	m0 := <-ch0
	assert.Equal(m0.WorkloadID, "workloadfortonictest")
	assert.True(strings.HasPrefix(string(m0.Data), "Get workload"))
	assert.Equal(m0.StdStreamType, types.EruError)

	store.On("GetWorkload", mock.Anything, mock.Anything).Return(workload, nil)

	engine.On("VirtualizationLogs", mock.Anything, mock.Anything).Return(nil, nil, fmt.Errorf("error")).Twice()
	_, ch1, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch1)
	m1 := <-ch1
	assert.Equal(m1.WorkloadID, "workloadfortonictest")
	assert.True(strings.HasPrefix(string(m1.Data), "Fetch log for workload"))
	assert.Equal(m1.StdStreamType, types.EruError)

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

	engine.On("VirtualizationWait", mock.Anything, mock.Anything, mock.Anything).Return(nil, fmt.Errorf("error"))
	ids, ch2, err := c.RunAndWait(context.Background(), opts, make(chan []byte))
	assert.NoError(err)
	assert.NotNil(ch2)
	assert.Equal(ids[0], "workloadfortonictest")
	assert.Equal(ids[1], "workloadfortonictest")

	ms := []*types.AttachWorkloadMessage{}
	for m := range ch2 {
		ms = append(ms, m)
	}
	assert.Equal(len(ms), 6)
	assert.Equal(ms[5].WorkloadID, "workloadfortonictest")
	assert.True(strings.HasPrefix(string(ms[5].Data), "Wait workload"))
	assert.Equal(ms[5].StdStreamType, types.EruError)
}

func newLambdaCluster(t *testing.T) (*Calcium, []*types.Node) {
	c, nodes := newCreateWorkloadCluster(t)

	store := &storemocks.Store{}
	sche := &schedulermocks.Scheduler{}
	scheduler.InitSchedulerV1(sche)
	c.store = store
	c.scheduler = sche

	node1, node2 := nodes[0], nodes[1]

	store.On("CreateProcessing", mock.Anything, mock.Anything, mock.Anything).Return(nil)
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
	sche.On("SelectStorageNodes", mock.Anything, mock.AnythingOfType("[]resourcetypes.ScheduleInfo"), mock.AnythingOfType("int64")).Return(func(_ context.Context, scheduleInfos []resourcetypes.ScheduleInfo, _ int64) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, len(nodes), nil)
	sche.On("SelectStorageNodes", mock.Anything, mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("int64")).Return(func(_ context.Context, scheduleInfos []resourcetypes.ScheduleInfo, _ int64) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, len(nodes), nil)
	sche.On("SelectVolumeNodes", mock.Anything, mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("types.VolumeBindings")).Return(func(_ context.Context, scheduleInfos []resourcetypes.ScheduleInfo, _ types.VolumeBindings) []resourcetypes.ScheduleInfo {
		return scheduleInfos
	}, nil, len(nodes), nil)
	sche.On("SelectMemoryNodes", mock.Anything, mock.AnythingOfType("[]types.ScheduleInfo"), mock.AnythingOfType("float64"), mock.AnythingOfType("int64")).Return(
		func(_ context.Context, scheduleInfos []resourcetypes.ScheduleInfo, _ float64, _ int64) []resourcetypes.ScheduleInfo {
			for i := range scheduleInfos {
				scheduleInfos[i].Capacity = 1
			}
			return scheduleInfos
		}, len(nodes), nil)

	store.On("MakeDeployStatus", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	old := strategy.Plans[strategy.Auto]
	strategy.Plans[strategy.Auto] = func(ctx context.Context, sis []strategy.Info, need, total, _ int) (map[string]int, error) {
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

	// doCreateAndStartWorkload fails: AddWorkload
	engine.On("VirtualizationCreate", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationCreated{ID: "workloadfortonictest"}, nil)
	engine.On("VirtualizationStart", mock.Anything, mock.Anything).Return(nil)
	engine.On("VirtualizationRemove", mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil)
	store.On("ListNodeWorkloads", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD)
	engine.On("VirtualizationInspect", mock.Anything, mock.Anything).Return(&enginetypes.VirtualizationInfo{}, nil)
	store.On("AddWorkload", mock.Anything, mock.Anything, mock.Anything).Return(nil)

	return c, nodes
}
