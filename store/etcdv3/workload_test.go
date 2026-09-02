package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

const (
	workloadID       = "1234567812345678123456781234567812345678123456781234567812345678"
	workloadName     = "test_app_1"
	workloadNodename = "n1"
	workloadPodname  = "test"
)

func TestAddORUpdateWorkload(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	workload := newWorkloadFixture()
	workload.Name = "a"
	err := m.AddWorkload(ctx, workload, nil)
	assert.Error(t, err)
	workload.Name = workloadName
	err = m.UpdateWorkload(ctx, workload)
	assert.Error(t, err)
	err = m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	err = m.UpdateWorkload(ctx, workload)
	assert.NoError(t, err)
	workload.Name = "test_app_2"
	err = m.UpdateWorkload(ctx, workload)
	assert.NoError(t, err)
}

func TestRemoveWorkload(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	workload := newWorkloadFixture()
	err := m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	workload.Name = "a"
	err = m.RemoveWorkload(ctx, workload)
	assert.Error(t, err)
	workload.Name = workloadName
	err = m.RemoveWorkload(ctx, workload)
	assert.NoError(t, err)
}

func TestGetWorkload(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	workload := newWorkloadFixture()
	err := m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	_, err = m.GetWorkloads(ctx, []string{workloadID, "xxx"})
	assert.Error(t, err)
	_, err = m.GetWorkload(ctx, workloadID)
	assert.Error(t, err)
	_, err = m.AddPod(ctx, workloadPodname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: workloadNodename, Endpoint: "mock://", Podname: workloadPodname})
	assert.NoError(t, err)
	_, err = m.GetWorkload(ctx, workloadID)
	assert.NoError(t, err)
}

func TestSetWorkloadStatus(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	workload := newWorkloadFixture()
	workload.StatusMeta = &types.StatusMeta{ID: workloadID}
	err := m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	assert.Error(t, err)

	workload.StatusMeta.Appname = "test"
	workload.StatusMeta.Entrypoint = "app"
	workload.StatusMeta.Nodename = workloadNodename
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.Equal(t, err, types.ErrInvaildCount)
	assert.NoError(t, m.AddWorkload(ctx, workload, nil))
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.NoError(t, err)
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.NoError(t, err)
	workload.StatusMeta.Running = true
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.NoError(t, err)
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	assert.NoError(t, err)
}

func TestListWorkloads(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	cs, err := m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	assert.NoError(t, err)
	assert.Empty(t, cs)
	workload := newWorkloadFixture()
	workload.Labels = map[string]string{"x": "y"}
	err = m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, workloadPodname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: workloadNodename, Endpoint: "mock://", Podname: workloadPodname})
	assert.NoError(t, err)
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, cs)
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, map[string]string{"x": "z"})
	assert.NoError(t, err)
	assert.Empty(t, cs)
}

func TestListNodeWorkloads(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	cs, err := m.ListNodeWorkloads(ctx, "", nil)
	assert.NoError(t, err)
	assert.Empty(t, cs)
	workload := newWorkloadFixture()
	workload.Labels = map[string]string{"x": "y"}
	err = m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, workloadPodname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: workloadNodename, Endpoint: "mock://", Podname: workloadPodname})
	assert.NoError(t, err)
	cs, err = m.ListNodeWorkloads(ctx, workloadNodename, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, cs)
	cs, err = m.ListNodeWorkloads(ctx, workloadNodename, map[string]string{"x": "z"})
	assert.NoError(t, err)
	assert.Empty(t, cs)
}

func TestWorkloadStatusStream(t *testing.T) {
	m := NewMercury(t)
	ctx := t.Context()
	appname := "test"
	entrypoint := "app"
	workload := &types.Workload{
		ID:         workloadID,
		Name:       workloadName,
		Nodename:   workloadNodename,
		Podname:    workloadPodname,
		StatusMeta: &types.StatusMeta{ID: workloadID},
	}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     workloadNodename,
			Podname:  workloadPodname,
			Endpoint: "tcp://127.0.0.1:2376",
		},
	}
	nodeBytes, err := json.Marshal(node)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, workloadPodname, "CPU")
	assert.NoError(t, err)
	_, err = kvOf(m).Create(ctx, fmt.Sprintf(common.NodeInfoKey, workloadNodename), string(nodeBytes))
	assert.NoError(t, err)
	_, err = kvOf(m).Create(ctx, fmt.Sprintf(common.NodePodKey, workloadPodname, workloadNodename), string(nodeBytes))
	assert.NoError(t, err)
	assert.NoError(t, m.AddWorkload(ctx, workload, nil))
	workload.StatusMeta = &types.StatusMeta{
		ID:         workloadID,
		Running:    true,
		Appname:    appname,
		Nodename:   workloadNodename,
		Entrypoint: entrypoint,
	}
	cctx, cancel := context.WithCancel(ctx)
	ch := m.WorkloadStatusStream(cctx, appname, entrypoint, "", nil)
	assert.NoError(t, m.SetWorkloadStatus(ctx, workload.StatusMeta, 0))
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()
	for s := range ch {
		assert.False(t, s.Delete)
		assert.NotNil(t, s.Workload)
	}
}

func newWorkloadFixture() *types.Workload {
	return &types.Workload{
		ID:       workloadID,
		Nodename: workloadNodename,
		Podname:  workloadPodname,
		Name:     workloadName,
	}
}
