package common

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/projecteru2/core/engine"
	enginefactory "github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/engine/fake"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (s *Store) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	if _, err := s.GetPod(ctx, opts.Podname); err != nil {
		return nil, err
	}

	data := map[string]string{}
	node := &types.Node{
		Name:      opts.Nodename,
		Endpoint:  opts.Endpoint,
		Podname:   opts.Podname,
		Labels:    opts.Labels,
		Available: true,
		Bypass:    false,
		Test:      opts.Test || strings.HasPrefix(opts.Endpoint, fakeengine.PrefixKey),
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	d := string(bytes)
	data[fmt.Sprintf(NodeInfoKey, opts.Nodename)] = d
	data[fmt.Sprintf(NodePodKey, opts.Podname, opts.Nodename)] = d

	if err := s.Create(ctx, data); err != nil {
		return nil, err
	}
	return node, nil
}

func (s *Store) RemoveNode(ctx context.Context, node *types.Node) error {
	if node == nil {
		return nil
	}

	err := s.Delete(ctx, []string{
		fmt.Sprintf(NodeInfoKey, node.Name),
		fmt.Sprintf(NodePodKey, node.Podname, node.Name),
	})
	log.WithFunc("store.common.RemoveNode").Infof(ctx, "node (%s, %s, %s) deleted", node.Podname, node.Name, node.Endpoint)
	return err
}

func (s *Store) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	nodes, err := s.GetNodes(ctx, []string{nodename})
	if err != nil {
		return nil, err
	}
	return nodes[0], nil
}

func (s *Store) GetNodes(ctx context.Context, nodenames []string) ([]*types.Node, error) {
	keys := make([]string, 0, len(nodenames))
	for _, nodename := range nodenames {
		keys = append(keys, fmt.Sprintf(NodeInfoKey, nodename))
	}

	kvs, err := s.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}
	return s.doGetNodes(ctx, kvs, nil, true, false)
}

func (s *Store) GetNodesByPod(ctx context.Context, nodeFilter *types.NodeFilter, withoutEngine bool) ([]*types.Node, error) {
	do := func(podname string) ([]*types.Node, error) {
		kvs, err := s.GetPrefix(ctx, fmt.Sprintf(NodePodKey, podname, ""), 0)
		if err != nil {
			return nil, err
		}
		return s.doGetNodes(ctx, kvs, nodeFilter.Labels, nodeFilter.All, withoutEngine)
	}
	if nodeFilter.Podname != "" {
		return do(nodeFilter.Podname)
	}
	pods, err := s.GetAllPods(ctx)
	if err != nil {
		return nil, err
	}
	result := []*types.Node{}
	for _, pod := range pods {
		ns, err := do(pod.Name)
		if err != nil {
			return nil, err
		}
		result = append(result, ns...)
	}
	return result, nil
}

func (s *Store) UpdateNodes(ctx context.Context, nodes ...*types.Node) error {
	data := map[string]string{}
	for _, node := range nodes {
		bytes, err := json.Marshal(node)
		if err != nil {
			return err
		}
		d := string(bytes)
		data[fmt.Sprintf(NodeInfoKey, node.Name)] = d
		data[fmt.Sprintf(NodePodKey, node.Podname, node.Name)] = d
	}
	return s.Put(ctx, data)
}

func (s *Store) SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error {
	if ttl == 0 {
		return types.ErrInvaildNodeStatusTTL
	}

	statusKey := filepath.Join(NodeStatusPrefix, node.Name)
	if ttl < 0 {
		return s.Delete(ctx, []string{statusKey})
	}

	data, err := json.Marshal(types.NodeStatus{
		Nodename: node.Name,
		Podname:  node.Podname,
		Alive:    true,
	})
	if err != nil {
		return err
	}

	return s.BindStatus(ctx, fmt.Sprintf(NodeInfoKey, node.Name), statusKey, string(data), ttl)
}

func (s *Store) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	data, err := s.GetOne(ctx, filepath.Join(NodeStatusPrefix, nodename))
	if err != nil {
		return nil, err
	}

	ns := &types.NodeStatus{}
	if err := json.Unmarshal([]byte(data), ns); err != nil {
		return nil, err
	}
	return ns, nil
}

func (s *Store) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	ch := make(chan *types.NodeStatus)
	logger := log.WithFunc("store.common.NodeStatusStream")
	utils.SentryGo(func() {
		defer func() {
			logger.Info(ctx, "close NodeStatusStream channel")
			close(ch)
		}()
		if err := s.nodeStatusStream(ctx, logger, ch); err != nil && ctx.Err() == nil {
			logger.Error(ctx, err, "node status stream interrupted")
		}
	})
	return ch
}

func (s *Store) MakeClient(ctx context.Context, node *types.Node) (engine.API, error) {
	if client := enginefactory.GetEngineFromCache(ctx, node.Endpoint); client != nil {
		return client, nil
	}
	return enginefactory.GetEngine(ctx, s.Config, node.Name, node.Endpoint)
}

func (s *Store) nodeStatusStream(ctx context.Context, logger *log.Fields, ch chan<- *types.NodeStatus) error {
	logger.Infof(ctx, "watch on %s", NodeStatusPrefix)
	for event := range s.Watch(ctx, NodeStatusPrefix) {
		nodename := utils.Tail(event.Key)
		status := &types.NodeStatus{
			Nodename: nodename,
			Alive:    event.Type == EventPut,
		}
		node, err := s.GetNode(ctx, nodename)
		if err != nil {
			status.Error = err
		} else {
			status.Podname = node.Podname
		}
		select {
		case ch <- status:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return types.ErrMessageChanClosed
}

func (s *Store) doGetNodes(
	ctx context.Context, kvs map[string]string,
	labels map[string]string, all, withoutEngine bool,
) (nodes []*types.Node, err error) {
	allNodes := []*types.Node{}
	for _, value := range kvs {
		node := &types.Node{}
		if err := json.Unmarshal([]byte(value), node); err != nil {
			return nil, err
		}
		if utils.LabelsFilter(node.Labels, labels) {
			allNodes = append(allNodes, node)
		}
	}
	logger := log.WithFunc("store.common.doGetNodes")

	var statuses map[string]string
	if len(allNodes) > 1 {
		var err error
		if statuses, err = s.GetPrefix(ctx, NodeStatusPrefix, 0); err != nil {
			logger.Error(ctx, err, "failed to list node statuses")
			statuses = nil
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(allNodes))
	mu := sync.Mutex{}
	nodes = make([]*types.Node, 0, len(allNodes))

	for _, node := range allNodes {
		_ = s.Pool.Invoke(func() {
			defer wg.Done()
			switch {
			case node.Test:
				node.Available = !node.Bypass
			case statuses != nil:
				_, node.Available = statuses[filepath.Join(NodeStatusPrefix, node.Name)]
			default:
				if _, err := s.GetNodeStatus(ctx, node.Name); err != nil && !s.NotFound(err) {
					logger.Errorf(ctx, err, "failed to get node status of %+v", node.Name)
				} else {
					node.Available = err == nil
				}
			}

			if !all && node.IsDown() {
				return
			}

			node.Engine = &fake.EngineWithErr{DefaultErr: types.ErrNilEngine, EP: enginetypes.NewParams(node.Name, node.Endpoint)}
			if !withoutEngine {
				if client, err := s.MakeClient(ctx, node); err != nil {
					logger.Errorf(ctx, err, "failed to make client for %+v", node.Name)
				} else {
					node.Engine = client
				}
			}
			mu.Lock()
			nodes = append(nodes, node)
			mu.Unlock()
		})
	}
	wg.Wait()

	return nodes, nil
}
