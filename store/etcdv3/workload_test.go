package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
)

func TestAddORUpdateWorkload(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     "a",
	}
	// failed by name
	err := m.AddWorkload(ctx, workload, nil)
	assert.Error(t, err)
	workload.Name = name
	// fail update
	err = m.UpdateWorkload(ctx, workload)
	assert.Error(t, err)
	// success create
	err = m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	// success updat
	err = m.UpdateWorkload(ctx, workload)
	assert.NoError(t, err)
	// success updat
	workload.Name = "test_app_2"
	err = m.UpdateWorkload(ctx, workload)
	assert.NoError(t, err)
}

func TestRemoveWorkload(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	// fail remove
	workload.Name = "a"
	err = m.RemoveWorkload(ctx, workload)
	assert.Error(t, err)
	workload.Name = name
	// success remove
	err = m.RemoveWorkload(ctx, workload)
	assert.NoError(t, err)
}

func TestGetWorkload(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	// failed by no workload
	_, err = m.GetWorkloads(ctx, []string{ID, "xxx"})
	assert.Error(t, err)
	// failed by no pod nodes
	_, err = m.GetWorkload(ctx, ID)
	assert.Error(t, err)
	// create pod node
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: "mock://", Podname: podname})
	assert.NoError(t, err)
	// success
	_, err = m.GetWorkload(ctx, ID)
	assert.NoError(t, err)
}

func TestGetWorkloadStatus(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	// failed no pod no node
	_, err = m.GetWorkloadStatus(ctx, ID)
	assert.Error(t, err)
	// add success
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: "mock://", Podname: podname})
	assert.NoError(t, err)
	c, err := m.GetWorkloadStatus(ctx, ID)
	assert.Nil(t, c)
}

func TestSetWorkloadStatus(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:         ID,
		Nodename:   nodename,
		Podname:    podname,
		StatusMeta: &types.StatusMeta{ID: ID},
	}
	// fail by no name
	err := m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	assert.Error(t, err)

	workload.Name = name
	workload.StatusMeta.Appname = "test"
	workload.StatusMeta.Entrypoint = "app"
	workload.StatusMeta.Nodename = "n1"
	// no workload, err nil
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.Equal(t, err, types.ErrEntityNotExists)
	assert.NoError(t, m.AddWorkload(ctx, workload, nil))
	// no status key, put succ, err nil
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.NoError(t, err)
	// status not changed, update old lease
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.NoError(t, err)
	// status changed, revoke old lease
	workload.StatusMeta.Running = true
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	assert.NoError(t, err)
	// status not changed, ttl = 0
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	assert.NoError(t, err)
}

func TestListWorkloads(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	// no key
	cs, err := m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	assert.NoError(t, err)
	assert.Empty(t, cs)
	// add workload
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	workload := &types.Workload{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
		Labels:   map[string]string{"x": "y"},
	}
	// success create
	err = m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: "mock://", Podname: podname})
	assert.NoError(t, err)
	// no labels
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, cs)
	// labels
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, map[string]string{"x": "z"})
	assert.NoError(t, err)
	assert.Empty(t, cs)
}

func TestListNodeWorkloads(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	// no key
	cs, err := m.ListNodeWorkloads(ctx, "", nil)
	assert.NoError(t, err)
	assert.Empty(t, cs)
	// add workload
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	workload := &types.Workload{
		ID:       ID,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
		Labels:   map[string]string{"x": "y"},
	}
	// success create
	err = m.AddWorkload(ctx, workload, nil)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, podname, "")
	assert.NoError(t, err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: "mock://", Podname: podname})
	assert.NoError(t, err)
	// no labels
	cs, err = m.ListNodeWorkloads(ctx, nodename, nil)
	assert.NoError(t, err)
	assert.NotEmpty(t, cs)
	// labels
	cs, err = m.ListNodeWorkloads(ctx, nodename, map[string]string{"x": "z"})
	assert.NoError(t, err)
	assert.Empty(t, cs)
}

func TestWorkloadStatusStream(t *testing.T) {
	m := NewMercury(t)
	ctx := context.Background()
	ID := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	appname := "test"
	entrypoint := "app"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:         ID,
		Name:       name,
		Nodename:   nodename,
		Podname:    podname,
		StatusMeta: &types.StatusMeta{ID: ID},
	}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     nodename,
			Podname:  podname,
			Endpoint: "tcp://127.0.0.1:2376",
		},
	}
	_, err := json.Marshal(workload)
	assert.NoError(t, err)
	nodeBytes, err := json.Marshal(node)
	assert.NoError(t, err)
	_, err = m.AddPod(ctx, podname, "CPU")
	assert.NoError(t, err)
	_, err = m.Create(ctx, fmt.Sprintf(nodeInfoKey, nodename), string(nodeBytes))
	assert.NoError(t, err)
	_, err = m.Create(ctx, fmt.Sprintf(nodePodKey, podname, nodename), string(nodeBytes))
	assert.NoError(t, err)
	assert.NoError(t, m.AddWorkload(ctx, workload, nil))
	// WorkloadStatusStream
	workload.StatusMeta = &types.StatusMeta{
		ID:         ID,
		Running:    true,
		Appname:    appname,
		Nodename:   nodename,
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
