package client

import (
	"context"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/client/interceptor"
	_ "github.com/projecteru2/core/client/resolver/eru"
	_ "github.com/projecteru2/core/client/resolver/static"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
)

// Client is a grpc connection to a core instance.
type Client struct {
	conn *grpc.ClientConn
}

func NewClient(_ context.Context, addr string, authConfig types.AuthConfig) (*Client, error) {
	cc, err := dial(addr, authConfig)
	return &Client{conn: cc}, err
}

func (c *Client) GetRPCClient() pb.CoreRPCClient {
	return pb.NewCoreRPCClient(c.conn)
}

func dial(addr string, authConfig types.AuthConfig) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 6 * 60 * time.Second, Timeout: time.Second}),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // initial policy only; the resolver's service config can override it, see https://github.com/grpc/grpc-go/blob/v1.40.x/examples/features/load_balancing/client/main.go
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 0})),
	}
	if authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(authConfig)))
	}

	return grpc.NewClient(addr, opts...)
}
