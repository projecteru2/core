package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"

	"github.com/cockroachdb/errors"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/engine"
	enginefactory "github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/engine/fake"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (m *Mercury) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	_, err := m.GetPod(ctx, opts.Podname)
	if err != nil {
		return nil, err
	}

	return m.doAddNode(ctx, opts.Nodename, opts.Endpoint, opts.Podname, opts.Ca, opts.Cert, opts.Key, opts.Labels, opts.Test)
}

func (m *Mercury) RemoveNode(ctx context.Context, node *types.Node) error {
	if node == nil {
		return nil
	}
	return m.doRemoveNode(ctx, node.Podname, node.Name, node.Endpoint)
}

func (m *Mercury) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	nodes, err := m.GetNodes(ctx, []string{nodename})
	if err != nil {
		return nil, err
	}
	return nodes[0], nil
}

func (m *Mercury) GetNodes(ctx context.Context, nodenames []string) ([]*types.Node, error) {
	nodesKeys := []string{}
	for _, nodename := range nodenames {
		key := fmt.Sprintf(nodeInfoKey, nodename)
		nodesKeys = append(nodesKeys, key)
	}

	kvs, err := m.GetMulti(ctx, nodesKeys)
	if err != nil {
		return nil, err
	}
	return m.doGetNodes(ctx, kvs, nil, true, nil)
}

func (m *Mercury) GetNodesByPod(ctx context.Context, nodeFilter *types.NodeFilter, opts ...store.Option) ([]*types.Node, error) {
	var op store.Op
	for _, opt := range opts {
		opt(&op)
	}
	do := func(podname string) ([]*types.Node, error) {
		key := fmt.Sprintf(nodePodKey, podname, "")
		resp, err := m.Get(ctx, key, clientv3.WithPrefix())
		if err != nil {
			return nil, err
		}
		return m.doGetNodes(ctx, resp.Kvs, nodeFilter.Labels, nodeFilter.All, &op)
	}
	if nodeFilter.Podname != "" {
		return do(nodeFilter.Podname)
	}
	pods, err := m.GetAllPods(ctx)
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

func (m *Mercury) UpdateNodes(ctx context.Context, nodes ...*types.Node) error {
	data := map[string]string{}
	addIfNotEmpty := func(key, value string) {
		if value != "" {
			data[key] = value
		}
	}
	for _, node := range nodes {
		bytes, err := json.Marshal(node)
		if err != nil {
			return err
		}
		d := string(bytes)
		data[fmt.Sprintf(nodeInfoKey, node.Name)] = d
		data[fmt.Sprintf(nodePodKey, node.Podname, node.Name)] = d
		addIfNotEmpty(fmt.Sprintf(nodeCaKey, node.Name), node.Ca)
		addIfNotEmpty(fmt.Sprintf(nodeCertKey, node.Name), node.Cert)
		addIfNotEmpty(fmt.Sprintf(nodeKeyKey, node.Name), node.Key)
	}

	resp, err := m.BatchPut(ctx, data)
	if err != nil {
		return err
	}
	if !resp.Succeeded {
		return types.ErrTxnConditionFailed
	}
	return nil
}

func (m *Mercury) SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error {
	if ttl == 0 {
		return types.ErrInvaildNodeStatusTTL
	}

	statusKey := filepath.Join(nodeStatusPrefix, node.Name)
	entityKey := fmt.Sprintf(nodeInfoKey, node.Name)

	if ttl < 0 {
		_, err := m.Delete(ctx, statusKey)
		return err
	}

	data, err := json.Marshal(types.NodeStatus{
		Nodename: node.Name,
		Podname:  node.Podname,
		Alive:    true,
	})
	if err != nil {
		return err
	}

	return m.BindStatus(ctx, entityKey, statusKey, string(data), ttl)
}

func (m *Mercury) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	key := filepath.Join(nodeStatusPrefix, nodename)
	ev, err := m.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	ns := &types.NodeStatus{}
	if err := json.Unmarshal(ev.Value, ns); err != nil {
		return nil, err
	}
	return ns, nil
}

func (m *Mercury) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	ch := make(chan *types.NodeStatus)
	logger := log.WithFunc("store.etcdv3.NodeStatusStream")
	_ = m.pool.Invoke(func() {
		defer func() {
			logger.Info(ctx, "close NodeStatusStream channel")
			close(ch)
		}()

		logger.Infof(ctx, "watch on %s", nodeStatusPrefix)
		for resp := range m.Watch(ctx, nodeStatusPrefix, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				if !resp.Canceled {
					logger.Error(ctx, resp.Err(), "watch failed")
				}
				return
			}
			for _, event := range resp.Events {
				nodename := extractNodename(string(event.Kv.Key))
				status := &types.NodeStatus{
					Nodename: nodename,
					Alive:    event.Type != clientv3.EventTypeDelete,
				}
				node, err := m.GetNode(ctx, nodename)
				if err != nil {
					status.Error = err
				} else {
					status.Podname = node.Podname
				}
				ch <- status
			}
		}
	})
	return ch
}

func (m *Mercury) LoadNodeCert(ctx context.Context, node *types.Node) (err error) {
	keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
	data := []string{"", "", ""}
	for i := range 3 {
		ev, err := m.GetOne(ctx, fmt.Sprintf(keyFormats[i], node.Name))
		if err != nil {
			if !errors.Is(err, types.ErrInvaildCount) {
				log.WithFunc("store.etcdv3.LoadNodeCert").Error(ctx, err, "get key")
				return err
			}
			continue
		}
		data[i] = string(ev.Value)
	}
	node.Ca, node.Cert, node.Key = data[0], data[1], data[2]
	return nil
}

func (m *Mercury) makeClient(ctx context.Context, node *types.Node) (client engine.API, err error) {
	// cache lookup ignores ca/cert/key
	if client = enginefactory.GetEngineFromCache(ctx, node.Endpoint, "", "", ""); client != nil {
		return client, nil
	}

	keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
	data := []string{"", "", ""}
	for i := range 3 {
		ev, err := m.GetOne(ctx, fmt.Sprintf(keyFormats[i], node.Name))
		if err != nil {
			if !errors.Is(err, types.ErrInvaildCount) {
				log.WithFunc("store.etcdv3.makeClient").Error(ctx, err, "get key")
				return nil, err
			}
			continue
		}
		data[i] = string(ev.Value)
	}

	return enginefactory.GetEngine(ctx, m.config, node.Name, node.Endpoint, data[0], data[1], data[2])
}

func (m *Mercury) doAddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, labels map[string]string, test bool) (*types.Node, error) {
	data := map[string]string{}
	if ca != "" {
		data[fmt.Sprintf(nodeCaKey, name)] = ca
	}
	if cert != "" {
		data[fmt.Sprintf(nodeCertKey, name)] = cert
	}
	if key != "" {
		data[fmt.Sprintf(nodeKeyKey, name)] = key
	}

	node := &types.Node{
		Name:      name,
		Endpoint:  endpoint,
		Podname:   podname,
		Labels:    labels,
		Available: true,
		Bypass:    false,
		Test:      test || strings.HasPrefix(endpoint, fakeengine.PrefixKey),
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	d := string(bytes)
	data[fmt.Sprintf(nodeInfoKey, name)] = d
	data[fmt.Sprintf(nodePodKey, podname, name)] = d

	resp, err := m.BatchCreate(ctx, data)
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded {
		return nil, types.ErrTxnConditionFailed
	}

	return node, nil
}

// certs are written before the node record, so a failed create leaves them behind
func (m *Mercury) doRemoveNode(ctx context.Context, podname, nodename, endpoint string) error {
	keys := []string{
		fmt.Sprintf(nodeInfoKey, nodename),
		fmt.Sprintf(nodePodKey, podname, nodename),
		fmt.Sprintf(nodeCaKey, nodename),
		fmt.Sprintf(nodeCertKey, nodename),
		fmt.Sprintf(nodeKeyKey, nodename),
	}

	_, err := m.BatchDelete(ctx, keys)
	log.WithFunc("store.etcdv3.doRemoveNode").Infof(ctx, "node (%s, %s, %s) deleted", podname, nodename, endpoint)
	return err
}

func (m *Mercury) doGetNodes(
	ctx context.Context, kvs []*mvccpb.KeyValue,
	labels map[string]string, all bool, op *store.Op,
) (nodes []*types.Node, err error) {
	allNodes := []*types.Node{}
	for _, ev := range kvs {
		node := &types.Node{}
		if err := json.Unmarshal(ev.Value, node); err != nil {
			return nil, err
		}
		ep := enginetypes.Params{
			Nodename: node.Name,
			Endpoint: node.Endpoint,
			CA:       node.Ca,
			Cert:     node.Cert,
			Key:      node.Key,
		}
		node.Engine = &fake.EngineWithErr{DefaultErr: types.ErrNilEngine, EP: &ep}
		if utils.LabelsFilter(node.Labels, labels) {
			allNodes = append(allNodes, node)
		}
	}
	logger := log.WithFunc("store.etcdv3.doGetNodes")

	wg := &sync.WaitGroup{}
	wg.Add(len(allNodes))
	nodesCh := make(chan *types.Node, len(allNodes))

	for _, node := range allNodes {
		_ = m.pool.Invoke(func() {
			defer wg.Done()
			if node.Test {
				node.Available = !node.Bypass
			} else if _, err := m.GetNodeStatus(ctx, node.Name); err != nil && !errors.Is(err, types.ErrInvaildCount) {
				logger.Errorf(ctx, err, "failed to get node status of %+v", node.Name)
			} else {
				node.Available = err == nil
			}

			if !all && node.IsDown() {
				return
			}

			if op == nil || (!op.WithoutEngine) {
				if client, err := m.makeClient(ctx, node); err != nil {
					logger.Errorf(ctx, err, "failed to make client for %+v", node.Name)
				} else {
					node.Engine = client
				}
			}
			nodesCh <- node
		})
	}
	wg.Wait()
	close(nodesCh)

	for node := range nodesCh {
		nodes = append(nodes, node)
	}

	return nodes, nil
}

func extractNodename(s string) string {
	ps := strings.Split(s, "/")
	return ps[len(ps)-1]
}
