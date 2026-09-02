package servicediscovery

import (
	"context"
	"net"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
)

func TestWatchFollowsThePushedAddresses(t *testing.T) {
	second := serveStatus(t)
	second.addresses = []string{second.addr}
	first := serveStatus(t)
	first.addresses = []string{second.addr}

	ctx, cancel := context.WithCancel(t.Context())
	ch, err := New(first.addr, types.AuthConfig{}).Watch(ctx)
	require.NoError(t, err)
	require.Equal(t, []string{second.addr}, <-ch)
	select {
	case <-second.calls:
	case <-time.After(5 * time.Second):
		t.Fatal("the watch did not follow the pushed addresses")
	}
	require.Equal(t, []string{second.addr}, <-ch)

	cancel()
	for range ch { //nolint:revive
	}
}

func TestWatchKeepsItsAddressesWhenThePushIsEmpty(t *testing.T) {
	server := serveStatus(t)
	server.addresses = []string{}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	_, err := New(server.addr, types.AuthConfig{}).Watch(ctx)
	require.NoError(t, err)
	for range 2 {
		select {
		case <-server.calls:
		case <-time.After(5 * time.Second):
			t.Fatal("the watch lost its addresses after an empty push")
		}
	}
}

type statusServer struct {
	pb.UnimplementedCoreRPCServer
	addr      string
	addresses []string
	calls     chan struct{}
}

func serveStatus(t *testing.T) *statusServer {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	s := &statusServer{addr: lis.Addr().String(), calls: make(chan struct{}, 16)}
	server := grpc.NewServer()
	pb.RegisterCoreRPCServer(server, s)
	go func() { _ = server.Serve(lis) }()
	t.Cleanup(server.Stop)
	return s
}

func (s *statusServer) WatchServiceStatus(_ *pb.Empty, stream pb.CoreRPC_WatchServiceStatusServer) error {
	s.calls <- struct{}{}
	return stream.Send(&pb.ServiceStatus{Addresses: s.addresses, IntervalInSecond: 60})
}
