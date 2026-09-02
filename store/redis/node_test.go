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
	time.Sleep(2 * time.Second)
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
	time.Sleep(2 * time.Second)
	s.advance(2 * time.Second)
	ns1, err := s.rediaron.GetNodeStatus(s.T().Context(), node.Name)
	s.Error(err)
	s.Nil(ns1)
}

func (s *RediaronTestSuite) TestNodeStatusStream() {
	node := s.addStatusNode()

	go func() {
		ctx, cancel := context.WithTimeout(s.T().Context(), 1000*time.Millisecond)
		defer cancel()
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			time.Sleep(500 * time.Millisecond)
			s.NoError(s.rediaron.SetNodeStatus(s.T().Context(), node, 1))
			triggerMockedKeyspaceNotification(s.rediaron.cli, filepath.Join(common.NodeStatusPrefix, node.Name), actionSet)
		}
	}()

	ctx, cancel := context.WithCancel(s.T().Context())
	ch := s.rediaron.NodeStatusStream(ctx)
	go func() {
		time.Sleep(1500 * time.Millisecond)
		triggerMockedKeyspaceNotification(s.rediaron.cli, filepath.Join(common.NodeStatusPrefix, node.Name), actionExpired)
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	statuses := []*types.NodeStatus{}
	for m := range ch {
		statuses = append(statuses, m)
	}
	for _, m := range statuses[:len(statuses)-1] {
		s.True(m.Alive)
	}
	s.False(statuses[len(statuses)-1].Alive)
}

func (s *RediaronTestSuite) addStatusNode() *types.Node {
	ctx := s.T().Context()
	_, err := s.rediaron.AddPod(ctx, "testpod", "")
	s.NoError(err)
	node, err := s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "testname", Endpoint: "mock://", Podname: "testpod"})
	s.NoError(err)
	return node
}
