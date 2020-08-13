package client

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/projecteru2/core/auth"
	"github.com/projecteru2/core/client/interceptor"
	_ "github.com/projecteru2/core/client/resolver/eru"
	_ "github.com/projecteru2/core/client/resolver/static"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"google.golang.org/grpc"
	"google.golang.org/grpc/keepalive"
)

// Client contain grpc conn
type Client struct {
	addr string
	conn *grpc.ClientConn
}

// NewClient new a client
func NewClient(ctx context.Context, addr string, authConfig types.AuthConfig) *Client {
	client := &Client{
		addr: addr,
		conn: dial(ctx, addr, authConfig),
	}
	return client
}

// GetConn return connection
func (c *Client) GetConn() *grpc.ClientConn {
	return c.conn
}

// GetRPCClient return rpc client
func (c *Client) GetRPCClient() pb.CoreRPCClient {
	return pb.NewCoreRPCClient(c.conn)
}

func dial(ctx context.Context, addr string, authConfig types.AuthConfig) *grpc.ClientConn {
	opts := []grpc.DialOption{
		grpc.WithInsecure(),
		grpc.WithKeepaliveParams(keepalive.ClientParameters{Time: 6 * 60 * time.Second, Timeout: time.Second}),
		grpc.WithBalancerName("round_robin"),
		grpc.WithUnaryInterceptor(interceptor.NewUnaryRetry(interceptor.RetryOptions{Max: 1})),
		grpc.WithStreamInterceptor(interceptor.NewStreamRetry(interceptor.RetryOptions{Max: 1})),
	}
	if authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(authConfig)))
	}

	target := utils.MakeTarget(addr)
	cc, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		log.Panicf("[NewClient] failed to dial grpc %s: %v", addr, err)
	}
	return cc
}
