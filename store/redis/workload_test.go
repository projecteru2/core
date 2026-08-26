package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

const (
	workloadID       = "1234567812345678123456781234567812345678123456781234567812345678"
	workloadName     = "test_app_1"
	workloadNodename = "n1"
	workloadPodname  = "test"
)

func (s *RediaronTestSuite) TestAddORUpdateWorkload() {
	ctx := s.T().Context()
	workload := s.newWorkloadFixture()
	workload.Name = "a"
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.Error(err)
	workload.Name = workloadName
	err = s.rediaron.UpdateWorkload(ctx, workload)
	s.Error(err)
	err = s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	err = s.rediaron.UpdateWorkload(ctx, workload)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestRemoveWorkload() {
	ctx := s.T().Context()
	workload := s.newWorkloadFixture()
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	workload.Name = "a"
	err = s.rediaron.RemoveWorkload(ctx, workload)
	s.Error(err)
	workload.Name = workloadName
	err = s.rediaron.RemoveWorkload(ctx, workload)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestGetWorkload() {
	ctx := s.T().Context()
	workload := s.newWorkloadFixture()
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	_, err = s.rediaron.GetWorkloads(ctx, []string{workloadID, "xxx"})
	s.Error(err)
	_, err = s.rediaron.GetWorkload(ctx, workloadID)
	s.Error(err)
	_, err = s.rediaron.AddPod(ctx, workloadPodname, "")
	s.NoError(err)
	_, err = s.rediaron.AddNode(ctx, &types.AddNodeOptions{
		Nodename: workloadNodename,
		Endpoint: "mock://",
		Podname:  workloadPodname,
	})
	s.NoError(err)
	_, err = s.rediaron.GetWorkload(ctx, workloadID)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestGetWorkloadStatus() {
	ctx := s.T().Context()
	workload := s.newWorkloadFixture()
	err := s.rediaron.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	_, err = s.rediaron.GetWorkloadStatus(ctx, workloadID)
	s.Error(err)
	_, err = s.rediaron.AddPod(ctx, workloadPodname, "")
	s.NoError(err)
	_, err = s.rediaron.AddNode(ctx, &types.AddNodeOptions{
		Nodename: workloadNodename,
		Endpoint: "mock://",
		Podname:  workloadPodname,
	})
	s.NoError(err)
	c, err := s.rediaron.GetWorkloadStatus(ctx, workloadID)
	s.NoError(err)
	s.Nil(c)
}

func (s *RediaronTestSuite) TestSetWorkloadStatus() {
	m := s.rediaron
	ctx := s.T().Context()
	workload := s.newWorkloadFixture()
	workload.StatusMeta = &types.StatusMeta{ID: workloadID}
	err := m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	s.Error(err)
	workload.StatusMeta.Appname = "test"
	workload.StatusMeta.Entrypoint = "app"
	workload.StatusMeta.Nodename = workloadNodename
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.ErrorIs(err, types.ErrInvaildCount)
	s.NoError(m.AddWorkload(ctx, workload, nil))
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.NoError(err)
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.NoError(err)
	workload.StatusMeta.Running = true
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 10)
	s.NoError(err)
	err = m.SetWorkloadStatus(ctx, workload.StatusMeta, 0)
	s.NoError(err)
}

func (s *RediaronTestSuite) TestListWorkloads() {
	m := s.rediaron
	ctx := s.T().Context()
	cs, err := m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	s.NoError(err)
	s.Empty(cs)
	workload := s.newWorkloadFixture()
	workload.Labels = map[string]string{"x": "y"}
	err = m.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	_, err = m.AddPod(ctx, workloadPodname, "")
	s.NoError(err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{
		Nodename: workloadNodename,
		Endpoint: "mock://",
		Podname:  workloadPodname,
	})
	s.NoError(err)
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, nil)
	s.NoError(err)
	s.NotEmpty(cs)
	cs, err = m.ListWorkloads(ctx, "", "a", "b", 1, map[string]string{"x": "z"})
	s.NoError(err)
	s.Empty(cs)
}

func (s *RediaronTestSuite) TestListNodeWorkloads() {
	m := s.rediaron
	ctx := s.T().Context()
	cs, err := m.ListNodeWorkloads(ctx, "", nil)
	s.NoError(err)
	s.Empty(cs)
	workload := s.newWorkloadFixture()
	workload.Labels = map[string]string{"x": "y"}
	err = m.AddWorkload(ctx, workload, nil)
	s.NoError(err)
	_, err = m.AddPod(ctx, workloadPodname, "")
	s.NoError(err)
	_, err = m.AddNode(ctx, &types.AddNodeOptions{Nodename: workloadNodename, Endpoint: "mock://", Podname: workloadPodname})
	s.NoError(err)
	cs, err = m.ListNodeWorkloads(ctx, workloadNodename, nil)
	s.NoError(err)
	s.NotEmpty(cs)
	cs, err = m.ListNodeWorkloads(ctx, workloadNodename, map[string]string{"x": "z"})
	s.NoError(err)
	s.Empty(cs)
}

func (s *RediaronTestSuite) TestWorkloadStatusStream() {
	m := s.rediaron
	ctx := s.T().Context()
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
	s.NoError(err)
	_, err = m.AddPod(ctx, workloadPodname, "CPU")
	s.NoError(err)
	err = m.Create(ctx, map[string]string{fmt.Sprintf(common.NodeInfoKey, workloadNodename): string(nodeBytes)})
	s.NoError(err)
	err = m.Create(ctx, map[string]string{fmt.Sprintf(common.NodePodKey, workloadPodname, workloadNodename): string(nodeBytes)})
	s.NoError(err)
	s.NoError(m.AddWorkload(ctx, workload, nil))
	workload.StatusMeta = &types.StatusMeta{
		ID:         workloadID,
		Running:    true,
		Appname:    appname,
		Entrypoint: entrypoint,
		Nodename:   workloadNodename,
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

func (s *RediaronTestSuite) newWorkloadFixture() *types.Workload {
	return &types.Workload{
		ID:       workloadID,
		Nodename: workloadNodename,
		Podname:  workloadPodname,
		Name:     workloadName,
	}
}
