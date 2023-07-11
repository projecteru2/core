package redis

import (
	"context"

	"github.com/projecteru2/core/types"
)

func (s *RediaronTestSuite) TestPod() {
	ctx := context.Background()
	podname := "testv3"

	pod, err := s.rediaron.AddPod(ctx, podname, "CPU")
	s.NoError(err)
	s.Equal(pod.Name, podname)

	_, err = s.rediaron.AddPod(ctx, podname, "CPU")
	s.Equal(err, ErrAlreadyExists)

	pod2, err := s.rediaron.GetPod(ctx, podname)
	s.NoError(err)
	s.Equal(pod2.Name, podname)

	pods, err := s.rediaron.GetAllPods(ctx)
	s.NoError(err)
	s.Equal(len(pods), 1)
	s.Equal(pods[0].Name, podname)

	_, err = s.rediaron.AddNode(ctx, &types.AddNodeOptions{Nodename: "test", Endpoint: "mock://", Podname: podname})
	s.NoError(err)
	err = s.rediaron.RemovePod(ctx, podname)
	s.Error(err)
	err = s.rediaron.RemoveNode(ctx, &types.Node{NodeMeta: types.NodeMeta{Podname: podname, Name: "test", Endpoint: "mock://"}})
	s.NoError(err)
	err = s.rediaron.RemovePod(ctx, podname)
	s.NoError(err)
}
