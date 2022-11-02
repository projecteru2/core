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
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// AddNode save it to etcd
// storage path in etcd is `/pod/nodes/:podname/:nodename`
// node->pod path in etcd is `/node/pod/:nodename`
// func (m *Mercury) AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string,
// cpu, share int, memory, storage int64, labels map[string]string,
// numa types.NUMA, numaMemory types.NUMAMemory, volume types.VolumeMap) (*types.Node, error) {
func (m *Mercury) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	_, err := m.GetPod(ctx, opts.Podname)
	if err != nil {
		return nil, err
	}

	return m.doAddNode(ctx, opts.Nodename, opts.Endpoint, opts.Podname, opts.Ca, opts.Cert, opts.Key, opts.Labels)
}

// RemoveNode delete a node
func (m *Mercury) RemoveNode(ctx context.Context, node *types.Node) error {
	if node == nil {
		return nil
	}
	return m.doRemoveNode(ctx, node.Podname, node.Name, node.Endpoint)
}

// GetNode get node by name
func (m *Mercury) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	nodes, err := m.GetNodes(ctx, []string{nodename})
	if err != nil {
		return nil, err
	}
	return nodes[0], nil
}

// GetNodes get nodes
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
	return m.doGetNodes(ctx, kvs, nil, true)
}

// GetNodesByPod get all nodes bound to pod
// here we use podname instead of pod instance
func (m *Mercury) GetNodesByPod(ctx context.Context, nodeFilter *types.NodeFilter) ([]*types.Node, error) {
	do := func(podname string) ([]*types.Node, error) {
		key := fmt.Sprintf(nodePodKey, podname, "")
		resp, err := m.Get(ctx, key, clientv3.WithPrefix())
		if err != nil {
			return nil, err
		}
		return m.doGetNodes(ctx, resp.Kvs, nodeFilter.Labels, nodeFilter.All)
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

// UpdateNodes .
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
		enginefactory.RemoveEngineFromCache(node.Endpoint, node.Ca, node.Cert, node.Key)
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

// SetNodeStatus sets status for a node, value will expire after ttl seconds
// ttl < 0 means to delete node status
// this is heartbeat of node
func (m *Mercury) SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error {
	if ttl == 0 {
		return types.ErrInvaildNodeStatusTTL
	}

	// nodenames are unique
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

// GetNodeStatus returns status for a node
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

// NodeStatusStream returns a stream of node status
// it tells you if status of a node is changed, either PUT or DELETE
// PUT    -> Alive: true
// DELETE -> Alive: false
func (m *Mercury) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	ch := make(chan *types.NodeStatus)
	_ = m.pool.Invoke(func() {
		defer func() {
			log.Info(ctx, "[NodeStatusStream] close NodeStatusStream channel")
			close(ch)
		}()

		log.Infof(ctx, "[NodeStatusStream] watch on %s", nodeStatusPrefix)
		for resp := range m.Watch(ctx, nodeStatusPrefix, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				if !resp.Canceled {
					log.Error(ctx, resp.Err(), "[NodeStatusStream] watch failed")
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

func (m *Mercury) makeClient(ctx context.Context, node *types.Node) (client engine.API, err error) {
	// try to get from cache without ca/cert/key
	if client = enginefactory.GetEngineFromCache(node.Endpoint, "", "", ""); client != nil {
		return client, nil
	}

	keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
	data := []string{"", "", ""}
	for i := 0; i < 3; i++ {
		ev, err := m.GetOne(ctx, fmt.Sprintf(keyFormats[i], node.Name))
		if err != nil {
			if !errors.Is(err, types.ErrInvaildCount) {
				log.Warnf(ctx, "[makeClient] Get key failed %+v", err)
				return nil, err
			}
			continue
		}
		data[i] = string(ev.Value)
	}

	return enginefactory.GetEngine(ctx, m.config, node.Name, node.Endpoint, data[0], data[1], data[2])
}

func (m *Mercury) doAddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, labels map[string]string) (*types.Node, error) {
	data := map[string]string{}
	// 如果有tls的证书需要保存就保存一下
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
		NodeMeta: types.NodeMeta{
			Name:     name,
			Endpoint: endpoint,
			Podname:  podname,
			Labels:   labels,
		},
		Available: true,
		Bypass:    false,
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

// 因为是先写etcd的证书再拿client
// 所以可能出现实际上node创建失败但是却写好了证书的情况
// 所以需要删除这些留存的证书
// 至于结果是不是成功就无所谓了
func (m *Mercury) doRemoveNode(ctx context.Context, podname, nodename, endpoint string) error {
	keys := []string{
		fmt.Sprintf(nodeInfoKey, nodename),
		fmt.Sprintf(nodePodKey, podname, nodename),
		fmt.Sprintf(nodeCaKey, nodename),
		fmt.Sprintf(nodeCertKey, nodename),
		fmt.Sprintf(nodeKeyKey, nodename),
	}

	_, err := m.BatchDelete(ctx, keys)
	log.Infof(ctx, "[doRemoveNode] Node (%s, %s, %s) deleted", podname, nodename, endpoint)
	return err
}

func (m *Mercury) doGetNodes(ctx context.Context, kvs []*mvccpb.KeyValue, labels map[string]string, all bool) (nodes []*types.Node, err error) {
	allNodes := []*types.Node{}
	for _, ev := range kvs {
		node := &types.Node{}
		if err := json.Unmarshal(ev.Value, node); err != nil {
			return nil, err
		}
		node.Engine = &fake.Engine{DefaultErr: types.ErrNilEngine}
		if utils.LabelsFilter(node.Labels, labels) {
			allNodes = append(allNodes, node)
		}
	}

	wg := &sync.WaitGroup{}
	wg.Add(len(allNodes))
	nodesCh := make(chan *types.Node, len(allNodes))

	for _, node := range allNodes {
		node := node
		_ = m.pool.Invoke(func() {
			defer wg.Done()
			if _, err := m.GetNodeStatus(ctx, node.Name); err != nil && !errors.Is(err, types.ErrInvaildCount) {
				log.Errorf(ctx, err, "[doGetNodes] failed to get node status of %+v", node.Name)
			} else {
				node.Available = err == nil
			}

			if !all && node.IsDown() {
				return
			}

			// update engine
			if client, err := m.makeClient(ctx, node); err != nil {
				log.Errorf(ctx, err, "[doGetNodes] failed to make client for %+v", node.Name)
			} else {
				node.Engine = client
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

// extracts node name from key
// /nodestatus/nodename -> nodename
func extractNodename(s string) string {
	ps := strings.Split(s, "/")
	return ps[len(ps)-1]
}
