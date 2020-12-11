package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/store"

	enginefactory "github.com/projecteru2/core/engine/factory"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
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

	// 尝试加载的客户端
	// 会自动判断是否是支持的 url
	client, err := enginefactory.GetEngine(ctx, m.config, opts.Nodename, opts.Endpoint, opts.Ca, opts.Cert, opts.Key)
	if err != nil {
		return nil, err
	}

	// 判断这货是不是活着的
	info, err := client.Info(ctx)
	if err != nil {
		return nil, err
	}
	// 更新默认值
	if opts.CPU == 0 {
		opts.CPU = info.NCPU
	}
	if opts.Memory == 0 {
		opts.Memory = info.MemTotal * 8 / 10 // use 80% real memory
	}
	if opts.Storage == 0 {
		opts.Storage = info.StorageTotal * 8 / 10
	}
	if opts.Share == 0 {
		opts.Share = m.config.Scheduler.ShareBase
	}
	if opts.Volume == nil {
		opts.Volume = types.VolumeMap{}
	}
	// 设置 numa 的内存默认值，如果没有的话，按照 numa node 个数均分
	if len(opts.Numa) > 0 {
		nodeIDs := map[string]struct{}{}
		for _, nodeID := range opts.Numa {
			nodeIDs[nodeID] = struct{}{}
		}
		perNodeMemory := opts.Memory / int64(len(nodeIDs))
		if opts.NumaMemory == nil {
			opts.NumaMemory = types.NUMAMemory{}
		}
		for nodeID := range nodeIDs {
			if _, ok := opts.NumaMemory[nodeID]; !ok {
				opts.NumaMemory[nodeID] = perNodeMemory
			}
		}
	}

	return m.doAddNode(ctx, opts.Nodename, opts.Endpoint, opts.Podname, opts.Ca, opts.Cert, opts.Key, opts.CPU, opts.Share, opts.Memory, opts.Storage, opts.Labels, opts.Numa, opts.NumaMemory, opts.Volume)
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
func (m *Mercury) GetNodesByPod(ctx context.Context, podname string, labels map[string]string, all bool) ([]*types.Node, error) {
	do := func(podname string) ([]*types.Node, error) {
		key := fmt.Sprintf(nodePodKey, podname, "")
		resp, err := m.Get(ctx, key, clientv3.WithPrefix())
		if err != nil {
			return nil, err
		}
		return m.doGetNodes(ctx, resp.Kvs, labels, all)
	}
	if podname != "" {
		return do(podname)
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
	for _, node := range nodes {
		bytes, err := json.Marshal(node)
		if err != nil {
			return errors.WithStack(err)
		}
		d := string(bytes)
		data[fmt.Sprintf(nodeInfoKey, node.Name)] = d
		data[fmt.Sprintf(nodePodKey, node.Podname, node.Name)] = d
	}

	_, err := m.batchUpdate(ctx, data)
	return errors.WithStack(err)
}

// UpdateNodeResource update cpu and memory on a node, either add or subtract
func (m *Mercury) UpdateNodeResource(ctx context.Context, node *types.Node, resource *types.ResourceMeta, action string) error {
	switch action {
	case store.ActionIncr:
		node.RecycleResources(resource)
	case store.ActionDecr:
		node.PreserveResources(resource)
	default:
		return types.ErrUnknownControlType
	}
	go metrics.Client.SendNodeInfo(node)
	return m.UpdateNodes(ctx, node)
}

func (m *Mercury) makeClient(ctx context.Context, node *types.Node, force bool) (engine.API, error) {
	// try get client, if nil, create a new one
	var client engine.API
	var err error
	client = _cache.Get(node.Name)
	if client == nil || force {
		keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
		data := []string{"", "", ""}
		for i := 0; i < 3; i++ {
			if ev, err := m.GetOne(ctx, fmt.Sprintf(keyFormats[i], node.Name)); err != nil {
				log.Warnf("[makeClient] Get key failed %v", err)
			} else {
				data[i] = string(ev.Value)
			}
		}

		client, err = enginefactory.GetEngine(ctx, m.config, node.Name, node.Endpoint, data[0], data[1], data[2])
		if err != nil {
			return nil, err
		}
		_cache.Set(node.Name, client)
	}
	return client, nil
}

func (m *Mercury) doAddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu, share int, memory, storage int64, labels map[string]string, numa types.NUMA, numaMemory types.NUMAMemory, volumemap types.VolumeMap) (*types.Node, error) {
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

	cpumap := types.CPUMap{}
	for i := 0; i < cpu; i++ {
		cpumap[strconv.Itoa(i)] = int64(share)
	}

	node := &types.Node{
		NodeMeta: types.NodeMeta{
			Name:           name,
			Endpoint:       endpoint,
			Podname:        podname,
			CPU:            cpumap,
			MemCap:         memory,
			StorageCap:     storage,
			Volume:         volumemap,
			InitCPU:        cpumap,
			InitMemCap:     memory,
			InitStorageCap: storage,
			InitNUMAMemory: numaMemory,
			InitVolume:     volumemap,
			Labels:         labels,
			NUMA:           numa,
			NUMAMemory:     numaMemory,
		},
		Available: true,
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	d := string(bytes)
	data[fmt.Sprintf(nodeInfoKey, name)] = d
	data[fmt.Sprintf(nodePodKey, podname, name)] = d

	_, err = m.batchCreate(ctx, data)
	if err != nil {
		return nil, err
	}

	go metrics.Client.SendNodeInfo(node)
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

	_cache.Delete(nodename)
	_, err := m.batchDelete(ctx, keys)
	log.Infof("[doRemoveNode] Node (%s, %s, %s) deleted", podname, nodename, endpoint)
	return err
}

func (m *Mercury) doGetNodes(ctx context.Context, kvs []*mvccpb.KeyValue, labels map[string]string, all bool) ([]*types.Node, error) {
	nodes := []*types.Node{}
	for _, ev := range kvs {
		node := &types.Node{}
		if err := json.Unmarshal(ev.Value, node); err != nil {
			return nil, err
		}
		node.Init()
		if (node.Available || all) && utils.FilterWorkload(node.Labels, labels) {
			engine, err := m.makeClient(ctx, node, false)
			if err != nil {
				return nil, err
			}
			node.Engine = engine
			nodes = append(nodes, node)
		}
	}
	return nodes, nil
}
