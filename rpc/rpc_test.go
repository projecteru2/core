package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"google.golang.org/grpc"
	grpcstatus "google.golang.org/grpc/status"

	grpcmocks "github.com/projecteru2/core/3rdmocks"
	clustermock "github.com/projecteru2/core/cluster/mocks"
	enginemock "github.com/projecteru2/core/engine/mocks"
	enginetypes "github.com/projecteru2/core/engine/types"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
)

func TestAddPod(t *testing.T) {
	v := newVibranium()
	opts := &pb.AddPodOptions{}
	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("AddPod", mock.Anything, mock.Anything, mock.Anything).Return(nil, types.ErrMockError).Once()
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
	).Return(nil, types.ErrMockError).Once()
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
	}
	opts, err := toCoreSetNodeOptions(b)
	assert.NoError(t, err)
	assert.Equal(t, "a", opts.Nodename)
}

func TestSetNodeTranformRejectsAMalformedResource(t *testing.T) {
	b := &pb.SetNodeOptions{
		Nodename:  "a",
		Resources: map[string][]byte{"cpumem": []byte("not json")},
	}
	_, err := toCoreSetNodeOptions(b)
	assert.Error(t, err)
}

func TestRunAndWaitSync(t *testing.T) {
	v := newVibranium()

	stream := &grpcmocks.BidiStreamingServer[pb.RunAndWaitOptions, pb.AttachWorkloadMessage]{}
	stream.On("Context").Return(context.Background())
	stream.On("Recv").Return(&pb.RunAndWaitOptions{
		DeployOptions: &pb.DeployOptions{
			Name: "deploy",
			Entrypoint: &pb.EntrypointOptions{
				Name:    "entry",
				Command: "ping",
			},
			Podname:   "pod",
			Image:     "image",
			OpenStdin: false,
			Resources: map[string][]byte{},
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

	stream := &grpcmocks.BidiStreamingServer[pb.RunAndWaitOptions, pb.AttachWorkloadMessage]{}
	stream.On("Context").Return(context.Background())
	stream.On("Recv").Return(&pb.RunAndWaitOptions{
		DeployOptions: &pb.DeployOptions{
			Name: "deploy",
			Entrypoint: &pb.EntrypointOptions{
				Name:    "entry",
				Command: "ping",
			},
			Podname:   "pod",
			Image:     "image",
			OpenStdin: false,
			Resources: map[string][]byte{},
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

func TestRemoveWorkloadReportsItsOwnStatusCode(t *testing.T) {
	v := newVibranium()

	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("RemoveWorkload", mock.Anything, mock.Anything, mock.Anything).
		Return(nil, types.ErrMockError).Once()

	err := v.RemoveWorkload(&pb.RemoveWorkloadOptions{IDs: []string{"id"}}, &removeWorkloadStream{})
	assert.Error(t, err)
	assert.Equal(t, RemoveWorkload, grpcstatus.Code(err))
}

func TestSendLargeFileReportsItsOwnStatusCode(t *testing.T) {
	v := newVibranium()

	stream := &grpcmocks.BidiStreamingServer[pb.FileOptions, pb.SendMessage]{}
	stream.On("Context").Return(context.Background())
	stream.On("Recv").Return(nil, types.ErrMockError)

	cluster := v.cluster.(*clustermock.Cluster)
	cluster.On("SendLargeFile", mock.Anything, mock.Anything).Return(
		func(_ context.Context, opts chan *types.SendLargeFileOptions) chan *types.SendMessage {
			ch := make(chan *types.SendMessage)
			go func() {
				defer close(ch)
				for range opts { //revive:disable-line:empty-block
				}
			}()
			return ch
		},
	)

	err := v.SendLargeFile(stream)
	assert.Error(t, err)
	assert.Equal(t, SendLargeFile, grpcstatus.Code(err))
}

func TestSendRejectsInvalidOptions(t *testing.T) {
	v := newVibranium()

	err := v.Send(&pb.SendOptions{}, &sendWorkloadStream{})
	assert.Error(t, err)
	assert.Equal(t, Send, grpcstatus.Code(err))
}

func TestReallocResourceStatusHidesStackTrace(t *testing.T) {
	v := newVibranium()

	_, err := v.ReallocResource(context.Background(), &pb.ReallocOptions{})
	assert.Error(t, err)
	assert.NotContains(t, grpcstatus.Convert(err).Message(), ".go:")
}

func newVibranium() *Vibranium {
	v := &Vibranium{
		cluster: &clustermock.Cluster{},
	}
	return v
}

type removeWorkloadStream struct {
	grpc.ServerStream
}

func (s *removeWorkloadStream) Send(*pb.RemoveWorkloadMessage) error { return nil }

func (s *removeWorkloadStream) Context() context.Context { return context.Background() }

type sendWorkloadStream struct {
	grpc.ServerStream
}

func (s *sendWorkloadStream) Send(*pb.SendMessage) error { return nil }

func (s *sendWorkloadStream) Context() context.Context { return context.Background() }
