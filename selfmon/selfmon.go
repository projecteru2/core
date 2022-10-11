package selfmon

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/pkg/errors"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// ActiveKey .
const ActiveKey = "/selfmon/active"

// NodeStatusWatcher monitors the changes of node status
type NodeStatusWatcher struct {
	id      int64
	config  types.Config
	cluster cluster.Cluster
	store   store.Store
}

// RunNodeStatusWatcher .
func RunNodeStatusWatcher(ctx context.Context, config types.Config, cluster cluster.Cluster, t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	id := rand.Int63n(10000) // nolint
	store, err := store.NewStore(config, t)
	if err != nil {
		log.Errorf(ctx, "[RunNodeStatusWatcher] %v failed to create store, err: %v", id, err)
		return
	}

	watcher := &NodeStatusWatcher{
		id:      id,
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
					log.Errorf(ctx, "[NodeStatusWatcher] %v stops watching, err: %v", n.id, err)
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

	var expiry <-chan struct{}
	var unregister func()
	defer func() {
		if unregister != nil {
			log.Infof(ctx, "[Register] %v unregisters", n.id)
			unregister()
		}
	}()

	retryCounter := 0

	for {
		select {
		case <-ctx.Done():
			log.Info("[Register] context canceled")
			return
		default:
		}

		// try to get the lock
		if ne, un, err := n.register(ctx); err != nil {
			if errors.Is(err, context.Canceled) {
				log.Info("[Register] context canceled")
				return
			} else if !errors.Is(err, types.ErrKeyExists) {
				log.Errorf(ctx, "[Register] failed to re-register: %v", err)
				time.Sleep(time.Second)
				continue
			}
			if retryCounter == 0 {
				log.Infof(ctx, "[Register] %v failed to register, there has been another active node status watcher", n.id)
			}
			retryCounter = (retryCounter + 1) % 60
			time.Sleep(time.Second)
		} else {
			log.Infof(ctx, "[Register] node status watcher %v has been active", n.id)
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
			log.Info("[Register] context canceled")
			return
		case <-expiry:
			log.Info("[Register] lock expired")
			return
		}
	}()

	f(ctx)
}

func (n *NodeStatusWatcher) register(ctx context.Context) (<-chan struct{}, func(), error) {
	return n.store.StartEphemeral(ctx, ActiveKey, n.config.HAKeepaliveInterval)
}

func (n *NodeStatusWatcher) initNodeStatus(ctx context.Context) {
	log.Debug(ctx, "[NodeStatusWatcher] init node status started")
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
				log.Errorf(ctx, "[NodeStatusWatcher] get pod nodes failed %v", err)
				return
			}
			for node := range ch {
				log.Debugf(ctx, "[NodeStatusWatcher] watched %s/%s", node.Name, node.Endpoint)
				nodes <- node
			}
		})
		if err != nil {
			log.Errorf(ctx, "[NodeStatusWatcher] get pod nodes failed %v", err)
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
		n.dealNodeStatusMessage(ctx, status)
	}
}

func (n *NodeStatusWatcher) monitor(ctx context.Context) error {
	// init node status first
	go n.initNodeStatus(ctx)

	// monitor node status
	messageChan := n.cluster.NodeStatusStream(ctx)
	log.Infof(ctx, "[NodeStatusWatcher] %v watch node status started", n.id)
	defer log.Infof(ctx, "[NodeStatusWatcher] %v stop watching node status", n.id)

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
	if message.Error != nil {
		log.Errorf(ctx, "[NodeStatusWatcher] deal with node status stream message failed %+v", message)
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
		log.Errorf(ctx, "[NodeStatusWatcher] set node %s failed %v", message.Nodename, err)
		return
	}
	log.Infof(ctx, "[NodeStatusWatcher] set node %s as alive: %v", message.Nodename, message.Alive)
}
