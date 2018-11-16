package client

import (
	"github.com/projecteru2/core/auth"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
)

// Client contain grpc conn
type Client struct {
	addr string
	conn *grpc.ClientConn
}

// NewClient new a client
func NewClient(addr string, authConfig types.AuthConfig) *Client {
	conn := connect(addr, authConfig)
	return &Client{addr: addr, conn: conn}
}

// GetConn return connection
func (c *Client) GetConn() *grpc.ClientConn {
	return c.conn
}

// GetRPCClient return rpc client
func (c *Client) GetRPCClient() pb.CoreRPCClient {
	return pb.NewCoreRPCClient(c.conn)
}

func connect(addr string, authConfig types.AuthConfig) *grpc.ClientConn {
	opts := []grpc.DialOption{grpc.WithInsecure()}
	if authConfig.Username != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(auth.NewCredential(authConfig)))
	}

	conn, err := grpc.Dial(addr, opts...)
	if err != nil {
		log.Fatalf("[ConnectEru] Can not connect %v", err)
	}
	log.Debugf("[ConnectEru] Init eru connection %s", addr)
	return conn
}
