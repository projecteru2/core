package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/projecteru2/core/engine"
	enginefactory "github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/engine/fake"
	"github.com/projecteru2/core/engine/mocks/fakeengine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/cockroachdb/errors"
)

// AddNode save it to etcd
// storage path in etcd is `/pod/nodes/:podname/:nodename`
// node->pod path in etcd is `/node/pod/:nodename`
// func (m *Rediaron) AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string,
// cpu, share int, memory, storage int64, labels map[string]string,
// numa types.NUMA, numaMemory types.NUMAMemory, volume types.VolumeMap) (*types.Node, error) {
func (r *Rediaron) AddNode(ctx context.Context, opts *types.AddNodeOptions) (*types.Node, error) {
	_, err := r.GetPod(ctx, opts.Podname)
	if err != nil {
		return nil, err
	}

	return r.doAddNode(ctx, opts.Nodename, opts.Endpoint, opts.Podname, opts.Ca, opts.Cert, opts.Key, opts.Labels, opts.Test)
}

// RemoveNode delete a node
func (r *Rediaron) RemoveNode(ctx context.Context, node *types.Node) error {
	if node == nil {
		return nil
	}
	return r.doRemoveNode(ctx, node.Podname, node.Name, node.Endpoint)
}

// GetNode get node by name
func (r *Rediaron) GetNode(ctx context.Context, nodename string) (*types.Node, error) {
	nodes, err := r.GetNodes(ctx, []string{nodename})
	if err != nil {
		return nil, err
	}
	return nodes[0], nil
}

// GetNodes get nodes
func (r *Rediaron) GetNodes(ctx context.Context, nodenames []string) ([]*types.Node, error) {
	nodesKeys := []string{}
	for _, nodename := range nodenames {
		key := fmt.Sprintf(nodeInfoKey, nodename)
		nodesKeys = append(nodesKeys, key)
	}

	kvs, err := r.GetMulti(ctx, nodesKeys)
	if err != nil {
		return nil, err
	}
	return r.doGetNodes(ctx, kvs, nil, true, nil)
}

// GetNodesByPod get all nodes bound to pod
// here we use podname instead of pod instance
func (r *Rediaron) GetNodesByPod(ctx context.Context, nodeFilter *types.NodeFilter, opts ...store.Option) ([]*types.Node, error) {
	op := store.NewOp(opts...)
	do := func(podname string) ([]*types.Node, error) {
		key := fmt.Sprintf(nodePodKey, podname, "*")
		kvs, err := r.getByKeyPattern(ctx, key, 0)
		if err != nil {
			return nil, err
		}
		return r.doGetNodes(ctx, kvs, nodeFilter.Labels, nodeFilter.All, op)
	}
	if nodeFilter.Podname != "" {
		return do(nodeFilter.Podname)
	}
	pods, err := r.GetAllPods(ctx)
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
func (r *Rediaron) UpdateNodes(ctx context.Context, nodes ...*types.Node) error {
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
	return r.BatchPut(ctx, data)
}

// SetNodeStatus sets status for a node, value will expire after ttl seconds
// ttl < 0 means delete node status
// this is heartbeat of node
func (r *Rediaron) SetNodeStatus(ctx context.Context, node *types.Node, ttl int64) error {
	if ttl == 0 {
		return types.ErrInvaildNodeStatusTTL
	}

	// nodenames are unique
	key := filepath.Join(nodeStatusPrefix, node.Name)

	if ttl < 0 {
		_, err := r.cli.Del(ctx, key).Result()
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

	_, err = r.cli.Set(ctx, key, string(data), time.Duration(ttl)*time.Second).Result()
	return err
}

// GetNodeStatus returns status for a node
func (r *Rediaron) GetNodeStatus(ctx context.Context, nodename string) (*types.NodeStatus, error) {
	key := filepath.Join(nodeStatusPrefix, nodename)
	ev, err := r.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	ns := &types.NodeStatus{}
	if err := json.Unmarshal([]byte(ev), ns); err != nil {
		return nil, err
	}
	return ns, nil
}

// NodeStatusStream returns a stream of node status
// it tells you if status of a node is changed, either PUT or DELETE
// PUT    -> Alive: true
// DELETE -> Alive: false
func (r *Rediaron) NodeStatusStream(ctx context.Context) chan *types.NodeStatus {
	ch := make(chan *types.NodeStatus)
	logger := log.WithFunc("store.redis.NodeStatusStream")
	_ = r.pool.Invoke(func() {
		defer func() {
			logger.Info(ctx, "close NodeStatusStream channel")
			close(ch)
		}()

		key := filepath.Join(nodeStatusPrefix, "*")
		logger.Infof(ctx, "watch on %s", key)
		for message := range r.KNotify(ctx, key) {
			nodename := extractNodename(message.Key)
			status := &types.NodeStatus{
				Nodename: nodename,
				Alive:    strings.ToLower(message.Action) != actionExpired,
			}
			node, err := r.GetNode(ctx, nodename)
			if err != nil {
				status.Error = err
			} else {
				status.Podname = node.Podname
			}
			ch <- status
		}
	})
	return ch
}

func (r *Rediaron) LoadNodeCert(ctx context.Context, node *types.Node) (err error) {
	keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
	data := []string{"", "", ""}
	for i := 0; i < 3; i++ {
		v, err := r.GetOne(ctx, fmt.Sprintf(keyFormats[i], node.Name))
		if err != nil {
			if !isRedisNoKeyError(err) {
				log.WithFunc("store.redis.LoadNodeCert").Warnf(ctx, "Get key failed %+v", err)
				return err
			}
			continue
		}
		data[i] = v
	}
	node.Ca, node.Cert, node.Key = data[0], data[1], data[2]
	return nil
}

func (r *Rediaron) makeClient(ctx context.Context, node *types.Node) (client engine.API, err error) {
	// try to get from cache without ca/cert/key
	if client = enginefactory.GetEngineFromCache(ctx, node.Endpoint, "", "", ""); client != nil {
		return client, nil
	}
	keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
	data := []string{"", "", ""}
	for i := 0; i < 3; i++ {
		v, err := r.GetOne(ctx, fmt.Sprintf(keyFormats[i], node.Name))
		if err != nil {
			if !isRedisNoKeyError(err) {
				log.WithFunc("store.redis.makeClient").Warnf(ctx, "Get key failed %+v", err)
				return nil, err
			}
			continue
		}
		data[i] = v
	}

	client, err = enginefactory.GetEngine(ctx, r.config, node.Name, node.Endpoint, data[0], data[1], data[2])
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (r *Rediaron) doAddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, labels map[string]string, test bool) (*types.Node, error) {
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
		Test:      test || strings.HasPrefix(endpoint, fakeengine.PrefixKey),
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	d := string(bytes)
	data[fmt.Sprintf(nodeInfoKey, name)] = d
	data[fmt.Sprintf(nodePodKey, podname, name)] = d

	err = r.BatchCreate(ctx, data)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// 因为是先写etcd的证书再拿client
// 所以可能出现实际上node创建失败但是却写好了证书的情况
// 所以需要删除这些留存的证书
// 至于结果是不是成功就无所谓了
func (r *Rediaron) doRemoveNode(ctx context.Context, podname, nodename, endpoint string) error {
	keys := []string{
		fmt.Sprintf(nodeInfoKey, nodename),
		fmt.Sprintf(nodePodKey, podname, nodename),
		fmt.Sprintf(nodeCaKey, nodename),
		fmt.Sprintf(nodeCertKey, nodename),
		fmt.Sprintf(nodeKeyKey, nodename),
	}

	err := r.BatchDelete(ctx, keys)
	log.WithFunc("store.redis.doRemoveNode").Infof(ctx, "Node (%s, %s, %s) deleted", podname, nodename, endpoint)
	return err
}

func (r *Rediaron) doGetNodes(
	ctx context.Context, kvs map[string]string,
	labels map[string]string, all bool, op *store.Op,
) (nodes []*types.Node, err error) {
	allNodes := []*types.Node{}
	for _, value := range kvs {
		node := &types.Node{}
		if err := json.Unmarshal([]byte(value), node); err != nil {
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
	logger := log.WithFunc("store.redis.doGetNodes")

	wg := &sync.WaitGroup{}
	wg.Add(len(allNodes))
	nodeChan := make(chan *types.Node, len(allNodes))

	for _, node := range allNodes {
		node := node
		_ = r.pool.Invoke(func() {
			defer wg.Done()
			if node.Test {
				node.Available = true && !node.Bypass
			} else if _, err := r.GetNodeStatus(ctx, node.Name); err != nil && !errors.Is(err, types.ErrInvaildCount) {
				logger.Errorf(ctx, err, "failed to get node status of %+v", node.Name)
			} else {
				node.Available = err == nil
			}

			if !all && node.IsDown() {
				return
			}

			nodeChan <- node
			if op == nil || (!op.WithoutEngine) {
				if client, err := r.makeClient(ctx, node); err != nil {
					logger.Errorf(ctx, err, "failed to make client for %+v", node.Name)
				} else {
					node.Engine = client
				}
			}
		})
	}
	wg.Wait()
	close(nodeChan)

	for node := range nodeChan {
		nodes = append(nodes, node)
	}

	return nodes, nil
}
