package client

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/projecteru2/core/log"
	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

type PoolConfig struct {
	EruAddrs          []string
	Auth              types.AuthConfig
	ConnectionTimeout time.Duration
}

type clientWithStatus struct {
	client pb.CoreRPCClient
	addr   string
	alive  bool
}

type Pool struct {
	mu         sync.Mutex
	rpcClients []*clientWithStatus
}

func NewCoreRPCClientPool(ctx context.Context, config *PoolConfig) (*Pool, error) {
	if len(config.EruAddrs) == 0 {
		return nil, types.ErrInvaildEruIPAddress
	}
	c := &Pool{rpcClients: []*clientWithStatus{}}
	for _, addr := range config.EruAddrs {
		var rpc *Client
		var err error
		utils.WithTimeout(ctx, config.ConnectionTimeout, func(ctx context.Context) {
			rpc, err = NewClient(ctx, addr, config.Auth)
		})
		if err != nil {
			log.WithFunc("client.NewCoreRPCClientPool").Errorf(ctx, err, "connect to %s failed", addr)
			continue
		}
		rpcClient := rpc.GetRPCClient()
		c.rpcClients = append(c.rpcClients, &clientWithStatus{client: rpcClient, addr: addr})
	}

	c.updateClientsStatus(ctx, config.ConnectionTimeout)

	if !slices.ContainsFunc(c.rpcClients, func(rpc *clientWithStatus) bool { return rpc.alive }) {
		return nil, types.ErrAllConnectionsFailed
	}

	go func() {
		ticker := time.NewTicker(config.ConnectionTimeout * 2)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				c.updateClientsStatus(ctx, config.ConnectionTimeout)
			case <-ctx.Done():
				return
			}
		}
	}()

	return c, nil
}

// GetClient returns the first alive client, or the first client when none are alive.
func (c *Pool) GetClient() pb.CoreRPCClient {
	c.mu.Lock()
	defer c.mu.Unlock()

	for _, rpc := range c.rpcClients {
		if rpc.alive {
			return rpc.client
		}
	}
	return c.rpcClients[0].client
}

func (c *Pool) updateClientsStatus(ctx context.Context, timeout time.Duration) {
	alive := make([]bool, len(c.rpcClients))
	var wg sync.WaitGroup
	for i, rpc := range c.rpcClients {
		wg.Go(func() {
			alive[i] = checkAlive(ctx, rpc, timeout)
		})
	}
	wg.Wait()

	c.mu.Lock()
	defer c.mu.Unlock()
	for i, rpc := range c.rpcClients {
		rpc.alive = alive[i]
	}
}

func checkAlive(ctx context.Context, rpc *clientWithStatus, timeout time.Duration) bool {
	var err error
	utils.WithTimeout(ctx, timeout, func(ctx context.Context) {
		_, err = rpc.client.Info(ctx, &pb.Empty{})
	})
	logger := log.WithFunc("client.checkAlive")
	if err != nil {
		logger.Warnf(ctx, "connect to %s failed: %+v", rpc.addr, err)
		return false
	}
	logger.Debugf(ctx, "connect to %s success", rpc.addr)
	return true
}
