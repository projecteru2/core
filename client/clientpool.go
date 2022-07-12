package client

import (
	"context"
	"sync"
	"time"

	pb "github.com/projecteru2/core/rpc/gen"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	log "github.com/sirupsen/logrus"
)

type clientWithStatus struct {
	client pb.CoreRPCClient
	addr   string
	alive  bool
}

// PoolConfig config for client pool
type PoolConfig struct {
	EruAddrs          []string
	Auth              types.AuthConfig
	ConnectionTimeout time.Duration
}

// Pool implement of RPCClientPool
type Pool struct {
	rpcClients []*clientWithStatus
}

// NewCoreRPCClientPool .
func NewCoreRPCClientPool(ctx context.Context, config *PoolConfig) (*Pool, error) {
	if len(config.EruAddrs) == 0 {
		return nil, types.ErrBadIPAddress
	}
	c := &Pool{rpcClients: []*clientWithStatus{}}
	for _, addr := range config.EruAddrs {
		var rpc *Client
		var err error
		utils.WithTimeout(ctx, config.ConnectionTimeout, func(ctx context.Context) {
			rpc, err = NewClient(ctx, addr, config.Auth)
		})
		if err != nil {
			log.Errorf("[NewCoreRPCClientPool] connect to %s failed, err: %s", addr, err)
			continue
		}
		rpcClient := rpc.GetRPCClient()
		c.rpcClients = append(c.rpcClients, &clientWithStatus{client: rpcClient, addr: addr})
	}

	// init client status
	c.updateClientsStatus(ctx, config.ConnectionTimeout)

	allFailed := true
	for _, rpc := range c.rpcClients {
		if rpc.alive {
			allFailed = false
		}
	}

	if allFailed {
		log.Error("[NewCoreRPCClientPool] all connections failed")
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

// GetClient finds the first *client.Client instance with an active connection. If all connections are dead, returns the first one.
func (c *Pool) GetClient() pb.CoreRPCClient {
	for _, rpc := range c.rpcClients {
		if rpc.alive {
			return rpc.client
		}
	}
	return c.rpcClients[0].client
}

func checkAlive(ctx context.Context, rpc *clientWithStatus, timeout time.Duration) bool {
	var err error
	utils.WithTimeout(ctx, timeout, func(ctx context.Context) {
		_, err = rpc.client.Info(ctx, &pb.Empty{})
	})
	if err != nil {
		log.Errorf("[ClientPool] connect to %s failed, err: %s", rpc.addr, err)
		return false
	}
	log.Debugf("[ClientPool] connect to %s success", rpc.addr)
	return true
}

func (c *Pool) updateClientsStatus(ctx context.Context, timeout time.Duration) {
	wg := &sync.WaitGroup{}
	for _, rpc := range c.rpcClients {
		wg.Add(1)
		go func(r *clientWithStatus) {
			defer wg.Done()
			r.alive = checkAlive(ctx, r, timeout)
		}(rpc)
	}
	wg.Wait()
}
