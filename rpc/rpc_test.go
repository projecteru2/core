package rpc

import (
	"context"
	"testing"

	clustermock "github.com/projecteru2/core/cluster/mocks"
	enginemock "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	pb "github.com/projecteru2/core/rpc/gen"
	rpcmocks "github.com/projecteru2/core/rpc/mocks"
	"github.com/projecteru2/core/types"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

func newVibranium() *Vibranium {
	v := &Vibranium{
		cluster: &clustermock.Cluster{},
	}
	return v
}

func TestAddPod(t *testing.T) {
	v := newVibranium()
	opts := &pb.AddPodOptions{}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("AddPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrNoETCD).Once()
	_, err := v.AddPod(context.Background(), opts)
	assert.Error(t, err)
	cluster.On("AddPod", mock.Anything, mock.Anything, mock.Anything).Return(&types.Pod{Name: "test", Desc: "test"}, nil)
	_, err = v.AddPod(context.Background(), opts)
	assert.NoError(t, err)
}

func TestAddNode(t *testing.T) {
	v := newVibranium()
	opts := &pb.AddNodeOptions{}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("AddNode", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(nil, types.ErrNoETCD).Once()
	_, err := v.AddNode(context.Background(), opts)
	assert.Error(t, err)
	engine := &enginemock.API{}
	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name: "test",
		},
		Engine: engine,
	}
	engine.On("Info", mock.Anything).Return(&enginetypes.Info{}, nil)
	cluster.On("AddNode", mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
		mock.Anything, mock.Anything, mock.Anything, mock.Anything,
	).Return(node, nil)
	_, err = v.AddNode(context.Background(), opts)
	assert.NoError(t, err)
}

func TestSetNodeTranform(t *testing.T) {
	b := &pb.SetNodeOptions{
		Nodename: "a",
		DeltaCpu: map[string]int32{"0": 1, "1": -1},
	}
	o, err := toCoreSetNodeOptions(b)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(o.DeltaCPU))
}

func TestRunAndWaitSync(t *testing.T) {
	v := newVibranium()

	stream := &rpcmocks.CoreRPC_RunAndWaitServer{}
	stream.On("Context").Return(context.Background())
	stream.On("Recv").Return(&pb.RunAndWaitOptions{
		DeployOptions: &pb.DeployOptions{
			Name: "deploy",
			Entrypoint: &pb.EntrypointOptions{
				Name:    "entry",
				Command: "ping",
			},
			Podname:      "pod",
			Image:        "image",
			OpenStdin:    false,
			ResourceOpts: &pb.ResourceOptions{},
		},
		Cmd:   []byte("ping"),
		Async: false,
	}, nil)

	rc := []*pb.AttachWorkloadMessage{}
	streamSendMock := func(m *pb.AttachWorkloadMessage) error {
		rc = append(rc, m)
		return nil
	}
	stream.On("Send", mock.Anything).Return(streamSendMock)

	runAndWait := func(_ context.Context, _ *types.DeployOptions, _ <-chan []byte) <-chan *types.AttachWorkloadMessage {
		ch := make(chan *types.AttachWorkloadMessage)
		go func() {
			// message to report output of process
			ch <- &types.AttachWorkloadMessage{
				WorkloadID:    "workloadidfortonic",
				Data:          []byte("network not reachable"),
				StdStreamType: types.Stderr,
			}
			close(ch)
		}()
		return ch
	}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("RunAndWait", mock.Anything, mock.Anything, mock.Anything).Return([]string{"workloadidfortonic"}, runAndWait, nil)

	err := v.RunAndWait(stream)
	assert.NoError(t, err)
	assert.Equal(t, len(rc), 2)

	m1 := rc[0]
	assert.Equal(t, m1.WorkloadId, "workloadidfortonic")
	assert.Equal(t, m1.Data, []byte(""))
	assert.Equal(t, m1.StdStreamType, pb.StdStreamType_TYPEWORKLOADID)

	m2 := rc[1]
	assert.Equal(t, m2.WorkloadId, "workloadidfortonic")
	assert.Equal(t, m2.Data, []byte("network not reachable"))
	assert.Equal(t, m2.StdStreamType, pb.StdStreamType_STDERR)
}

func TestRunAndWaitAsync(t *testing.T) {
	v := newVibranium()

	stream := &rpcmocks.CoreRPC_RunAndWaitServer{}
	stream.On("Context").Return(context.Background())
	stream.On("Recv").Return(&pb.RunAndWaitOptions{
		DeployOptions: &pb.DeployOptions{
			Name: "deploy",
			Entrypoint: &pb.EntrypointOptions{
				Name:    "entry",
				Command: "ping",
			},
			Podname:      "pod",
			Image:        "image",
			OpenStdin:    false,
			ResourceOpts: &pb.ResourceOptions{},
		},
		Cmd:   []byte("ping"),
		Async: true,
	}, nil)

	rc := []*pb.AttachWorkloadMessage{}
	streamSendMock := func(m *pb.AttachWorkloadMessage) error {
		rc = append(rc, m)
		return nil
	}
	stream.On("Send", mock.Anything).Return(streamSendMock)

	runAndWait := func(_ context.Context, _ *types.DeployOptions, _ <-chan []byte) <-chan *types.AttachWorkloadMessage {
		ch := make(chan *types.AttachWorkloadMessage)
		go func() {
			// message to report output of process
			ch <- &types.AttachWorkloadMessage{
				WorkloadID:    "workloadidfortonic",
				Data:          []byte("network not reachable"),
				StdStreamType: types.Stderr,
			}
			close(ch)
		}()
		return ch
	}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("RunAndWait", mock.Anything, mock.Anything, mock.Anything).Return([]string{"workloadidfortonic"}, runAndWait, nil)

	err := v.RunAndWait(stream)
	assert.NoError(t, err)
	assert.Equal(t, len(rc), 1)

	m1 := rc[0]
	assert.Equal(t, m1.WorkloadId, "workloadidfortonic")
	assert.Equal(t, m1.Data, []byte(""))
	assert.Equal(t, m1.StdStreamType, pb.StdStreamType_TYPEWORKLOADID)
}
