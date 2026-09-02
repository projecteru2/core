package selfmon

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

const (
	ActiveKey = "/selfmon/active"

	nodeStatusHandlers = 16
)

// NodeStatusWatcher watches node status changes.
type NodeStatusWatcher struct {
	ID      int64
	config  types.Config
	cluster cluster.Cluster
	store   store.Store
	wal     wal.WAL
}

func (n *NodeStatusWatcher) run(ctx context.Context) {
	for ctx.Err() == nil {
		n.withActiveLock(ctx, func(ctx context.Context) {
			go n.replayDeadJournals(ctx)
			if err := n.monitor(ctx); err != nil {
				log.WithFunc("selfmon.run").WithField("ID", n.ID).Error(ctx, err, "stops watching node status")
			}
		})
		select {
		case <-ctx.Done():
			return
		case <-time.After(n.config.ConnectionTimeout):
		}
	}
}

func (n *NodeStatusWatcher) withActiveLock(parentCtx context.Context, f func(ctx context.Context)) {
	ctx, cancel := context.WithCancel(parentCtx)
	defer cancel()
	logger := log.WithFunc("selfmon.withActiveLock").WithField("ID", n.ID)

	var expiry <-chan struct{}
	var unregister func()
	defer func() {
		if unregister != nil {
			logger.Info(ctx, "unregisters")
			unregister()
		}
	}()

	retryInterval := max(time.Second, n.config.HAKeepaliveInterval/4)
	warnEvery := max(1, int(time.Minute/retryInterval))
	retryCounter := 0

	for {
		ne, un, err := n.store.StartEphemeral(ctx, ActiveKey, n.config.HAKeepaliveInterval)
		if err == nil {
			logger.Info(ctx, "node status watcher has been active")
			expiry = ne
			unregister = un
			break
		}
		if errors.Is(err, context.Canceled) {
			logger.Info(ctx, "context canceled")
			return
		}
		switch {
		case !errors.Is(err, types.ErrKeyExists):
			logger.Error(ctx, err, "failed to register")
		case retryCounter == 0:
			logger.Warn(ctx, "failed to register, there has been another active node status watcher")
		}
		retryCounter = (retryCounter + 1) % warnEvery
		select {
		case <-ctx.Done():
			logger.Info(ctx, "context canceled")
			return
		case <-time.After(retryInterval):
		}
	}

	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
			logger.Info(ctx, "context canceled")
			return
		case <-expiry:
			logger.Warn(ctx, "active lock expired")
			return
		}
	}()

	f(ctx)
}

// replayDeadJournals hands the journals of unregistered instances to this one, for as long as it stays active.
func (n *NodeStatusWatcher) replayDeadJournals(ctx context.Context) {
	logger := log.WithFunc("selfmon.replayDeadJournals").WithField("ID", n.ID)
	_ = utils.KeepAlive(ctx, n.config.GRPCConfig.ServiceHeartbeatInterval, func(ctx context.Context) error {
		live, err := n.store.GetServiceStatus(ctx)
		if err != nil {
			logger.Error(ctx, err, "failed to read service status")
			return nil
		}
		if len(live) == 0 {
			return nil
		}
		n.wal.Takeover(ctx, live)
		return nil
	})
}

func (n *NodeStatusWatcher) initNodeStatus(ctx context.Context) {
	logger := log.WithFunc("selfmon.initNodeStatus")
	logger.Debug(ctx, "init node status started")
	nodes := make(chan *types.Node)

	go func() {
		defer close(nodes)
		utils.WithTimeout(ctx, n.config.GlobalTimeout, func(ctx context.Context) {
			ch, err := n.cluster.ListPodNodes(ctx, &types.ListNodesOptions{All: true})
			if err != nil {
				logger.Error(ctx, err, "get pod nodes failed")
				return
			}
			for node := range ch {
				logger.Debugf(ctx, "watched %s/%s", node.Name, node.Endpoint)
				nodes <- node
			}
		})
	}()

	var handlers errgroup.Group
	handlers.SetLimit(nodeStatusHandlers)
	for node := range nodes {
		status := &types.NodeStatus{Nodename: node.Name, Podname: node.Podname, Alive: node.Available || node.Test}
		handlers.Go(func() error {
			n.dealNodeStatusMessage(ctx, status)
			return nil
		})
	}
	_ = handlers.Wait()
}

func (n *NodeStatusWatcher) monitor(ctx context.Context) error {
	go n.initNodeStatus(ctx)
	logger := log.WithFunc("selfmon.monitor").WithField("ID", n.ID)

	messageChan := n.cluster.NodeStatusStream(ctx)
	logger.Info(ctx, "watch node status started")
	defer logger.Info(ctx, "stop watching node status")

	var handlers errgroup.Group
	handlers.SetLimit(nodeStatusHandlers)
	defer func() { _ = handlers.Wait() }()
	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				return types.ErrMessageChanClosed
			}
			handlers.Go(func() error {
				n.dealNodeStatusMessage(ctx, message)
				return nil
			})
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

func (n *NodeStatusWatcher) dealNodeStatusMessage(ctx context.Context, message *types.NodeStatus) {
	logger := log.WithFunc("selfmon.dealNodeStatusMessage")
	if message.Error != nil {
		logger.Errorf(ctx, message.Error, "deal with node status stream message failed %+v", message)
		return
	}
	// the agent owns the transition back to alive
	if message.Alive {
		return
	}

	opts := &types.SetNodeOptions{
		Nodename:      message.Nodename,
		WorkloadsDown: true,
	}
	if _, err := n.cluster.SetNode(ctx, opts); err != nil {
		logger.Errorf(ctx, err, "set node %s failed", message.Nodename)
		return
	}
	logger.Infof(ctx, "set node %s workloads down", message.Nodename)
}

func RunNodeStatusWatcher(ctx context.Context, config types.Config, cluster cluster.Cluster, store store.Store, journal wal.WAL) {
	watcher := &NodeStatusWatcher{
		ID:      rand.Int64N(10000), //nolint:gosec // a log-only instance tag, not a security token
		config:  config,
		store:   store,
		cluster: cluster,
		wal:     journal,
	}
	watcher.run(ctx)
}
