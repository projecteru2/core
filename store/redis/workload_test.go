package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestAddORUpdateWorkload() {
	ctx := context.Background()
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       id,
		Nodename: nodename,
		Podname:  podname,
		Name:     "a",
	}
	// failed by name
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.Error(err)
	workload.Name = name
	// fail update
	err = s.rediaron.UpdateWorkload(ctx, workload)
	s.Error(err)
	// success create
	err = s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	// success updat
	err = s.rediaron.UpdateWorkload(ctx, workload)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestRemoveWorkload() {
	ctx := context.Background()
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       id,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	// fail remove
	workload.Name = "a"
	err = s.rediaron.RemoveWorkload(ctx, workload)
	s.Error(err)
	workload.Name = name
	// success remove
	err = s.rediaron.RemoveWorkload(ctx, workload)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestGetWorkload() {
	ctx := context.Background()
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       id,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	// failed by no workload
	_, err = s.rediaron.GetWorkloads(ctx, []string{id, "xxx"})
	s.Error(err)
	// failed by no pod nodes
	_, err = s.rediaron.GetWorkload(ctx, id)
	s.Error(err)
	// create pod node
	_, err = s.rediaron.AddPod(ctx, podname, "")
	s.NoError(err)
	_, err = s.rediaron.AddNode(ctx, &types.AddNodeOptions{
		Nodename: nodename,
		Endpoint: "mock://",
		Podname:  podname,
	})
	s.NoError(err)
	// success
	_, err = s.rediaron.GetWorkload(ctx, id)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestGetWorkloadStatus() {
	ctx := context.Background()
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:       id,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
	}
	// success create
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	// failed no pod no node
	_, err = s.rediaron.GetWorkloadStatus(ctx, id)
	s.Error(err)
	// add success
	_, err = s.rediaron.AddPod(ctx, podname, "")
	s.NoError(err)
	_, err = s.rediaron.AddNode(ctx, &types.AddNodeOptions{
		Nodename: nodename,
		Endpoint: "mock://",
		Podname:  podname,
	})
	s.NoError(err)
	c, err := s.rediaron.GetWorkloadStatus(ctx, id)
	s.Nil(c)
}

func (s *RediaronTestSuite) TestSetWorkloadStatus() {
	m := s.rediaron
	ctx := context.Background()
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:         id,
		Nodename:   nodename,
		Podname:    podname,
		StatusMeta: &types.StatusMeta{ID: id},
	}
	// fail by no name
	err := m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	s.Error(err)
	workload.Name = name
	workload.StatusMeta.Appname = "test"
	workload.StatusMeta.Entrypoint = "app"
	workload.StatusMeta.Nodename = "n1"
	// no workload, err nil
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.ErrorIs(err, types.ErrEntityNotExists)
	s.NoError(m.AddWorkload(ctx, workload, nil))
	// no status key, put succ, err nil
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.NoError(err)
	// status not changed, update old lease
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.NoError(err)
	// status changed, revoke old lease
	workload.StatusMeta.Running = true
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.NoError(err)
	// status not changed, ttl = 0
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestListWorkloads() {
	m := s.rediaron
	ctx := context.Background()
	// no key
	cs, err := m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	s.NoError(err)
	s.Empty(cs)
	// add workload
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	workload := &types.Workload{
		ID:       id,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
		Labels:   map[string]string{"x": "y"},
	}
	// success create
	err = m.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	_, err = m.AddPod(ctx, podname, "")
	s.NoError(err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{
		Nodename: nodename,
		Endpoint: "mock://",
		Podname:  podname,
	})
	s.NoError(err)
	// no labels
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	s.NoError(err)
	s.NotEmpty(cs)
	// labels
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, map[string]string{"x": "z"})
	s.NoError(err)
	s.Empty(cs)
}

func (s *RediaronTestSuite) TestListNodeWorkloads() {
	m := s.rediaron
	ctx := context.Background()
	// no key
	cs, err := m.ListNodeWorkloads(ctx, "", nil)
	s.NoError(err)
	s.Empty(cs)
	// add workload
	name := "test_app_1"
	nodename := "n1"
	podname := "test"
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	workload := &types.Workload{
		ID:       id,
		Nodename: nodename,
		Podname:  podname,
		Name:     name,
		Labels:   map[string]string{"x": "y"},
	}
	// success create
	err = m.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	_, err = m.AddPod(ctx, podname, "")
	s.NoError(err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename, Endpoint: "mock://", Podname: podname})
	s.NoError(err)
	// no labels
	cs, err = m.ListNodeWorkloads(ctx, nodename, nil)
	s.NoError(err)
	s.NotEmpty(cs)
	// labels
	cs, err = m.ListNodeWorkloads(ctx, nodename, map[string]string{"x": "z"})
	s.NoError(err)
	s.Empty(cs)
}

func (s *RediaronTestSuite) TestWorkloadStatusStream() {
	m := s.rediaron
	ctx := context.Background()
	id := "1234567812345678123456781234567812345678123456781234567812345678"
	name := "test_app_1"
	appname := "test"
	entrypoint := "app"
	nodename := "n1"
	podname := "test"
	workload := &types.Workload{
		ID:         id,
		Name:       name,
		Nodename:   nodename,
		Podname:    podname,
		StatusMeta: &types.StatusMeta{ID: id},
	}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     nodename,
			Podname:  podname,
			Endpoint: "tcp://127.0.0.1:2376",
		},
	}
	_, err := json.Marshal(workload)
	s.NoError(err)
	nodeBytes, err := json.Marshal(node)
	s.NoError(err)
	_, err = m.AddPod(ctx, podname, "CPU")
	s.NoError(err)
	err = m.BatchCreate(ctx, map[string]string{fmt.Sprintf(nodeInfoKey, nodename): string(nodeBytes)})
	s.NoError(err)
	err = m.BatchCreate(ctx, map[string]string{fmt.Sprintf(nodePodKey, podname, nodename): string(nodeBytes)})
	s.NoError(err)
	s.NoError(m.AddWorkload(ctx, workload, nil))
	// WorkloadStatusStream
	workload.StatusMeta = &types.StatusMeta{
		ID:         id,
		Running:    true,
		Appname:    appname,
		Entrypoint: entrypoint,
		Nodename:   nodename,
	}
	cctx, cancel := context.WithCancel(ctx)
	ch := m.WorkloadStatusStream(cctx, appname, entrypoint, "", nil)
	s.NoError(m.SetWorkloadStatus(ctx, workload.StatusMeta, 0))
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()
	for st := range ch {
		s.False(st.Delete)
		s.NotNil(st.Workload)
	}
}
