package redis

import (
	"context"
	"path/filepath"
	"time"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestAddNode() {
	ctx := s.T().Context()
	podname := "testpod"
	_, err := s.rediaron.AddPod(ctx, podname, "test")
	s.NoError(err)
	_, err = s.rediaron.AddPod(ctx, "numapod", "test")
	s.NoError(err)
	s.rediaron.Config.Scheduler.ShareBase = 100
	labels := map[string]string{"test": "1"}

	nodename3 := "nodename3"
	endpoint3 := "tcp://path"
	node3, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: nodename3, Endpoint: endpoint3, Podname: podname, Labels: labels})
	s.NoError(err)
	_, err = s.rediaron.MakeClient(ctx, node3)
	s.Error(err)
}

func (s *RediaronTestSuite) TestRemoveNode() {
	ctx := s.T().Context()
	_, err := s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	node, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod"})
	s.NoError(err)
	s.Equal(node.Name, "test")
	s.NoError(s.rediaron.RemoveNode(ctx, nil))
	s.NoError(s.rediaron.RemoveNode(ctx, node))
}

func (s *RediaronTestSuite) TestGetNode() {
	ctx := s.T().Context()
	_, err := s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	node, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod"})
	s.NoError(err)
	s.Equal(node.Name, "test")
	_, err = s.rediaron.GetNode(ctx, "wtf")
	s.Error(err)
	n, err := s.rediaron.GetNode(ctx, "test")
	s.NoError(err)
	s.Equal(node.Name, n.Name)
}

func (s *RediaronTestSuite) TestGetNodesByPod() {
	ctx := s.T().Context()
	_, err := s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	node, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod", Labels: map[string]string{"x": "y"}})
	s.NoError(err)
	s.Equal(node.Name, "test")
	ns, err := s.rediaron.GetNodesByPod(ctx, &types.NodeFilter{Podname: "wtf", All: false}, false)
	s.NoError(err)
	s.Empty(ns)
	ns, err = s.rediaron.GetNodesByPod(ctx, &types.NodeFilter{Podname: "testpod", All: true}, false)
	s.NoError(err)
	s.NotEmpty(ns)
	ns, err = s.rediaron.GetNodesByPod(ctx, &types.NodeFilter{All: false}, false)
	s.NoError(err)
	s.Len(ns, 1)
	ns, err = s.rediaron.GetNodesByPod(ctx, &types.NodeFilter{All: true}, false)
	s.NoError(err)
	s.NotEmpty(ns)
}

func (s *RediaronTestSuite) TestUpdateNode() {
	ctx := s.T().Context()
	_, err := s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	node, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: "testpod", Labels: map[string]string{"x": "y"}})
	s.NoError(err)
	s.Equal(node.Name, "test")
	fakeNode := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "nil",
			Podname:  "wtf",
			Endpoint: "mock://hh",
		},
	}
	s.NoError(s.rediaron.UpdateNodes(ctx, fakeNode))
	s.NoError(s.rediaron.UpdateNodes(ctx, node))
}

func (s *RediaronTestSuite) TestSetNodeStatus() {
	node := s.addStatusNode()
	s.NoError(s.rediaron.SetNodeStatus(s.T().Context(), node, 1))
	key := filepath.Join(common.NodeStatusPrefix, node.Name)

	_, err := s.rediaron.GetOne(s.T().Context(), key)
	s.NoError(err)
	s.advance(2 * time.Second)
	_, err = s.rediaron.GetOne(s.T().Context(), key)
	s.Error(err)
}

func (s *RediaronTestSuite) TestSetNodeStatusOfAnUnknownNode() {
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:     "testname",
			Endpoint: "ep",
			Podname:  "testpod",
		},
	}
	s.Error(s.rediaron.SetNodeStatus(s.T().Context(), node, 1))
}

func (s *RediaronTestSuite) TestGetNodeStatus() {
	node := s.addStatusNode()
	s.NoError(s.rediaron.SetNodeStatus(s.T().Context(), node, 1))

	ns, err := s.rediaron.GetNodeStatus(s.T().Context(), node.Name)
	s.NoError(err)
	s.Equal(ns.Nodename, node.Name)
	s.True(ns.Alive)
	s.advance(2 * time.Second)
	ns1, err := s.rediaron.GetNodeStatus(s.T().Context(), node.Name)
	s.Error(err)
	s.Nil(ns1)
}

func (s *RediaronTestSuite) TestNodeStatusStream() {
	node := s.addStatusNode()
	key := filepath.Join(common.NodeStatusPrefix, node.Name)
	ctx, cancel := context.WithCancel(s.T().Context())
	defer cancel()

	ch := s.rediaron.NodeStatusStream(ctx)
	s.NoError(s.rediaron.SetNodeStatus(ctx, node, 1))
	statuses := []*types.NodeStatus{s.awaitSubscribedStatus(ctx, ch, key)}

	s.NoError(s.rediaron.SetNodeStatus(ctx, node, 1))
	triggerMockedKeyspaceNotification(ctx, s.rediaron.cli, key, actionSet)
	triggerMockedKeyspaceNotification(ctx, s.rediaron.cli, key, actionExpired)
	for statuses[len(statuses)-1].Alive {
		statuses = append(statuses, s.nextStatus(ch))
	}
	cancel()

	for _, m := range statuses[:len(statuses)-1] {
		s.True(m.Alive)
	}
	s.False(statuses[len(statuses)-1].Alive)
	select {
	case _, ok := <-ch:
		s.False(ok)
	case <-time.After(5 * time.Second):
		s.FailNow("node status stream did not close")
	}
}

func (s *RediaronTestSuite) addStatusNode() *types.Node {
	ctx := s.T().Context()
	_, err := s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	node, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "testname", Endpoint: "mock://", Podname: "testpod"})
	s.NoError(err)
	return node
}

func (s *RediaronTestSuite) awaitSubscribedStatus(ctx context.Context, ch <-chan *types.NodeStatus, key string) *types.NodeStatus {
	deadline := time.Now().Add(5 * time.Second)
	for {
		triggerMockedKeyspaceNotification(ctx, s.rediaron.cli, key, actionSet)
		select {
		case status, ok := <-ch:
			s.Require().True(ok)
			return status
		case <-time.After(5 * time.Millisecond):
			s.Require().True(time.Now().Before(deadline), "node status stream never subscribed")
		}
	}
}

func (s *RediaronTestSuite) nextStatus(ch <-chan *types.NodeStatus) *types.NodeStatus {
	select {
	case status, ok := <-ch:
		s.Require().True(ok)
		return status
	case <-time.After(5 * time.Second):
		s.Require().FailNow("node status stream delivered no status")
		return nil
	}
}
