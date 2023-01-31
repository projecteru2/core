package client

import (
	"context"
	"time"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/client/interceptor"
	_ "github.com/projecteru2/core/client/resolver/eru"    // register grpc resolver: eru://
	_ "github.com/projecteru2/core/client/resolver/static" // register grpc resolver: static://
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/keepalive"
)

// Client contain grpc conn
type Client struct {
	addr string
	conn *grpc.ClientConn
}

// NewClient new a client
func NewClient(ctx context.Context, addr string, authConfig types.AuthConfig) (*Client, error) {
	cc, err := dial(ctx, addr, authConfig)
	return &Client{
		addr: addr,
		conn: cc,
	}, err
}

// GetConn return connection
func (c *Client) GetConn() *grpc.ClientConn {
	return c.conn
}

// GetRPCClient return rpc client
func (c *Client) GetRPCClient() pb.CoreRPCClient {
	return pb.NewCoreRPCClient(c.conn)
}

func dial(ctx context.Context, addr string, authConfig types.AuthConfig) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 6 * 60 * time.Second, Timeout: time.Second}),
		grpc.WithDefaultServiceConfig(`{"loadBalancingPolicy":"round_robin"}`), // This sets the initial balancing policy, see https://github.com/grpc/grpc-go/blob/v1.40.x/examples/features/load_balancing/client/main.go
		grpc.WithUnaryInterceptor(interceptor.NewUnaryRetry(interceptor.RetryOptions{Max: 0})),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 0})),
	}
	if authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(authConfig)))
	}

	return grpc.DialContext(ctx, addr, opts...)
}
