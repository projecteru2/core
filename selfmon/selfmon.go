package selfmon

import (
	"context"
	"math/rand/v2"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"github.com/projecteru2/core/wal"
)

const ActiveKey = "/selfmon/active"

// NodeStatusWatcher watches node status changes.
type NodeStatusWatcher struct {
	ID      int64
	config  types.Config
	cluster cluster.Cluster
	store   store.Store
	wal     wal.WAL
}

func (n *NodeStatusWatcher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n.withActiveLock(ctx, func(ctx context.Context) {
				go n.replayDeadJournals(ctx)
				if err := n.monitor(ctx); err != nil {
					log.WithFunc("selfmon.run").WithField("ID", n.ID).Error(ctx, err, "stops watching node status")
				}
			})
			time.Sleep(n.config.ConnectionTimeout)
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

	retryCounter := 0

	for {
		select {
		case <-ctx.Done():
			logger.Info(ctx, "context canceled")
			return
		default:
		}

		if ne, un, err := n.store.StartEphemeral(ctx, ActiveKey, n.config.HAKeepaliveInterval); err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info(ctx, "context canceled")
				return
			} else if !errors.Is(err, types.ErrKeyExists) {
				logger.Error(ctx, err, "failed to register")
				time.Sleep(time.Second)
				continue
			}
			if retryCounter == 0 {
				logger.Warn(ctx, "failed to register, there has been another active node status watcher")
			}
			retryCounter = (retryCounter + 1) % 60
			time.Sleep(time.Second)
		} else {
			logger.Info(ctx, "node status watcher has been active")
			expiry = ne
			unregister = un
			break
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
	ticker := time.NewTicker(n.config.GRPCConfig.ServiceHeartbeatInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			live, err := n.store.GetServiceStatus(ctx)
			if err != nil {
				logger.Error(ctx, err, "failed to read service status")
				continue
			}
			n.wal.Takeover(ctx, live)
		}
	}
}

func (n *NodeStatusWatcher) initNodeStatus(ctx context.Context) {
	logger := log.WithFunc("selfmon.initNodeStatus")
	logger.Debug(ctx, "init node status started")
	nodes := make(chan *types.Node)

	go func() {
		defer close(nodes)
		utils.WithTimeout(ctx, n.config.GlobalTimeout, func(ctx context.Context) {
			ch, err := n.cluster.ListPodNodes(ctx, &types.ListNodesOptions{
				Podname:  "",
				Labels:   nil,
				All:      true,
				CallInfo: false,
			})
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

	for node := range nodes {
		status, err := n.cluster.GetNodeStatus(ctx, node.Name)
		if err != nil {
			status = &types.NodeStatus{
				Nodename: node.Name,
				Podname:  node.Podname,
				Alive:    false,
			}
		}
		if node.Test {
			status.Alive = true
		}
		n.dealNodeStatusMessage(ctx, status)
	}
}

func (n *NodeStatusWatcher) monitor(ctx context.Context) error {
	go n.initNodeStatus(ctx)
	logger := log.WithFunc("selfmon.monitor").WithField("ID", n.ID)

	messageChan := n.cluster.NodeStatusStream(ctx)
	logger.Info(ctx, "watch node status started")
	defer logger.Info(ctx, "stop watching node status")

	for {
		select {
		case message, ok := <-messageChan:
			if !ok {
				return types.ErrMessageChanClosed
			}
			go n.dealNodeStatusMessage(ctx, message)
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
