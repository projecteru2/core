package selfmon

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// ActiveKey .
const ActiveKey = "/selfmon/active"

// NodeStatusWatcher monitors the changes of node status
type NodeStatusWatcher struct {
	ID      int64
	config  types.Config
	cluster cluster.Cluster
	store   store.Store
}

// RunNodeStatusWatcher .
func RunNodeStatusWatcher(ctx context.Context, config types.Config, cluster cluster.Cluster, t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	ID := rand.Int63n(10000) //nolint
	store, err := store.NewStore(config, t)
	if err != nil {
		log.WithFunc("selfmon.RunNodeStatusWatcher").WithField("ID", ID).Error(ctx, err, "failed to create store")
		return
	}

	watcher := &NodeStatusWatcher{
		ID:      ID,
		config:  config,
		store:   store,
		cluster: cluster,
	}
	watcher.run(ctx)
}

func (n *NodeStatusWatcher) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
			n.withActiveLock(ctx, func(ctx context.Context) {
				if err := n.monitor(ctx); err != nil {
					log.WithFunc("selfmon.run").Errorf(ctx, err, "stops watching node id %+v", n.ID)
				}
			})
			time.Sleep(n.config.ConnectionTimeout)
		}
	}
}

// withActiveLock acquires the active lock synchronously
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

		// try to get the lock
		if ne, un, err := n.register(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				logger.Info(ctx, "context canceled")
				return
			} else if !errors.Is(err, types.ErrKeyExists) {
				logger.Error(ctx, err, "failed to re-register")
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

	// cancel the ctx when: 1. selfmon closed 2. lost the active lock
	go func() {
		defer cancel()

		select {
		case <-ctx.Done():
			logger.Info(ctx, "context canceled")
			return
		case <-expiry:
			logger.Info(ctx, "lock expired")
			return
		}
	}()

	f(ctx)
}

func (n *NodeStatusWatcher) register(ctx context.Context) (<-chan struct{}, func(), error) {
	return n.store.StartEphemeral(ctx, ActiveKey, n.config.HAKeepaliveInterval)
}

func (n *NodeStatusWatcher) initNodeStatus(ctx context.Context) {
	logger := log.WithFunc("selfmon.initNodeStatus")
	logger.Debug(ctx, "init node status started")
	nodes := make(chan *types.Node)

	go func() {
		defer close(nodes)
		// Get all nodes which are active status, and regardless of pod.
		var err error
		var ch <-chan *types.Node
		utils.WithTimeout(ctx, n.config.GlobalTimeout, func(ctx context.Context) {
			ch, err = n.cluster.ListPodNodes(ctx, &types.ListNodesOptions{
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
		if err != nil {
			logger.Error(ctx, err, "get pod nodes failed")
			return
		}
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
		// deal with fakeengine
		if strings.HasPrefix(node.Endpoint, fakeengine.PrefixKey) {
			status.Alive = true
		}
		n.dealNodeStatusMessage(ctx, status)
	}
}

func (n *NodeStatusWatcher) monitor(ctx context.Context) error {
	// init node status first
	go n.initNodeStatus(ctx)
	logger := log.WithFunc("selfmon.monitor").WithField("ID", n.ID)

	// monitor node status
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
	// here we ignore node back to alive status because it will updated by agent
	if message.Alive {
		return
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// TODO maybe we need a distributed lock to control concurrency
	opts := &types.SetNodeOptions{
		Nodename:      message.Nodename,
		WorkloadsDown: true,
	}
	if _, err := n.cluster.SetNode(ctx, opts); err != nil {
		logger.Errorf(ctx, err, "set node %s failed", message.Nodename)
		return
	}
	logger.Infof(ctx, "set node %s as alive: %+v", message.Nodename, message.Alive)
}
