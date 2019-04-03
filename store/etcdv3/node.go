package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"strconv"
	"strings"

	"github.com/projecteru2/core/store"

	"github.com/coreos/etcd/clientv3"
	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/metrics"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// AddNode save it to etcd
// storage path in etcd is `/pod/nodes/:podname/:nodename`
// node->pod path in etcd is `/node/pod/:nodename`
func (m *Mercury) AddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu, share int, memory int64, labels map[string]string) (*types.Node, error) {
	// TODO VM branch
	if !strings.HasPrefix(endpoint, nodeTCPPrefixKey) &&
		!strings.HasPrefix(endpoint, nodeSockPrefixKey) &&
		!strings.HasPrefix(endpoint, nodeMockPrefixKey) {
		return nil, types.NewDetailedErr(types.ErrNodeFormat,
			fmt.Sprintf("endpoint must starts with %s or %s %q",
				nodeTCPPrefixKey, nodeSockPrefixKey, endpoint))
	}

	_, err := m.GetPod(ctx, podname)
	if err != nil {
		return nil, err
	}

	if _, err := m.doGetNode(ctx, podname, name); err == nil {
		return nil, types.NewDetailedErr(types.ErrNodeExist,
			fmt.Sprintf("node %s:%s already exists",
				podname, name))
	}

	// 尝试加载的客户端
	// 会自动判断是否是支持的 url
	engine, err := m.doMakeClient(ctx, name, endpoint, ca, cert, key)
	if err != nil {
		return nil, err
	}

	// 判断这货是不是活着的
	info, err := engine.Info(ctx)
	if err != nil {
		return nil, err
	}
	// 更新默认值
	if cpu == 0 {
		cpu = info.NCPU
	}
	if memory == 0 {
		memory = info.MemTotal - types.GByte
	}
	if share == 0 {
		share = m.config.Scheduler.ShareBase
	}

	return m.doAddNode(ctx, name, endpoint, podname, ca, cert, key, cpu, share, memory, labels)
}

// DeleteNode delete a node
func (m *Mercury) DeleteNode(ctx context.Context, node *types.Node) {
	if node == nil {
		return
	}
	m.doDeleteNode(ctx, node.Podname, node.Name, node.Endpoint)
}

// GetNode get a node from etcd
// and construct it's engine client
// a node must belong to a pod
// and since node is not the smallest unit to user, to get a node we must specify the corresponding pod
// storage path in etcd is `/pod/nodes/:podname/:nodename`
func (m *Mercury) GetNode(ctx context.Context, podname, nodename string) (*types.Node, error) {
	node, err := m.doGetNode(ctx, podname, nodename)
	if err != nil {
		return nil, err
	}

	engine, err := m.makeClient(ctx, podname, nodename, node.Endpoint, false)
	if err != nil {
		return nil, err
	}

	node.Engine = engine
	return node, nil
}

// GetNodeByName get node by name
// first get podname from `/node/pod/:nodename`
// then call GetNode
func (m *Mercury) GetNodeByName(ctx context.Context, nodename string) (*types.Node, error) {
	key := fmt.Sprintf(nodePodKey, nodename)
	ev, err := m.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	podname := string(ev.Value)
	return m.GetNode(ctx, podname, nodename)
}

// GetNodesByPod get all nodes bound to pod
// here we use podname instead of pod instance
// storage path in etcd is `/pod/nodes/:podname`
func (m *Mercury) GetNodesByPod(ctx context.Context, podname string) ([]*types.Node, error) {
	key := fmt.Sprintf(podNodesKey, podname)
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithKeysOnly())
	if err != nil {
		return []*types.Node{}, err
	}

	nodes := []*types.Node{}
	for _, ev := range resp.Kvs {
		nodename := utils.Tail(string(ev.Key))
		n, err := m.GetNode(ctx, podname, nodename)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, err
}

// GetAllNodes get all nodes from etcd
// any error will break and return immediately
func (m *Mercury) GetAllNodes(ctx context.Context) ([]*types.Node, error) {
	pods, err := m.GetAllPods(ctx)
	if err != nil {
		return []*types.Node{}, err
	}

	nodes := []*types.Node{}
	for _, pod := range pods {
		ns, err := m.GetNodesByPod(ctx, pod.Name)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, ns...)
	}
	return nodes, err
}

// UpdateNode update a node, save it to etcd
// storage path in etcd is `/pod/nodes/:podname/:nodename`
func (m *Mercury) UpdateNode(ctx context.Context, node *types.Node) error {
	key := fmt.Sprintf(nodeInfoKey, node.Podname, node.Name)
	bytes, err := json.Marshal(node)
	if err != nil {
		return err
	}

	value := string(bytes)
	log.Debugf("[UpdateNode] pod %s node %s cpu slots %v mem %v", node.Podname, node.Name, node.CPU, node.MemCap)
	_, err = m.Put(ctx, key, value)
	return err
}

// UpdateNodeResource update cpu and mem on a node, either add or subtract
func (m *Mercury) UpdateNodeResource(ctx context.Context, node *types.Node, cpu types.CPUMap, quota float64, mem int64, action string) error {
	switch action {
	case store.ActionIncr:
		node.CPU.Add(cpu)
		node.SetCPUUsed(quota, types.DecrUsage)
		node.MemCap += mem
	case store.ActionDecr:
		node.CPU.Sub(cpu)
		node.SetCPUUsed(quota, types.IncrUsage)
		node.MemCap -= mem
	default:
		return types.ErrUnknownControlType
	}

	go metrics.Client.SendNodeInfo(node)
	return m.UpdateNode(ctx, node)
}

func (m *Mercury) makeClient(ctx context.Context, podname, nodename, endpoint string, force bool) (engine.API, error) {
	// try get client, if nil, create a new one
	var client engine.API
	var err error
	client = _cache.Get(nodename)
	if client == nil || force {
		var ca, cert, key string
		if m.config.Docker.CertPath != "" {
			keyFormats := []string{nodeCaKey, nodeCertKey, nodeKeyKey}
			data := []string{"", "", ""}
			for i := 0; i < 3; i++ {
				ev, err := m.GetOne(ctx, fmt.Sprintf(keyFormats[i], nodename))
				if err != nil {
					log.Warnf("[makeClient] Get key failed %v", err)
					data[i] = ""
				} else {
					data[i] = string(ev.Value)
				}
			}
			ca = data[0]
			cert = data[1]
			key = data[2]
		}
		client, err = m.doMakeClient(ctx, nodename, endpoint, ca, cert, key)
		if err != nil {
			return nil, err
		}
		_cache.Set(nodename, client)
	}
	return client, nil
}

func (m *Mercury) doMakeClient(ctx context.Context, nodename, endpoint, ca, cert, key string) (engine.API, error) {
	// TODO VM branch
	// like HasPrefix(endpoint,  nodeVMPrefix)
	// do connect to VM agent
	if strings.HasPrefix(endpoint, nodeMockPrefixKey) {
		return makeMockClient()
	}
	if strings.HasPrefix(endpoint, nodeSockPrefixKey) ||
		m.config.Docker.CertPath == "" ||
		(ca == "" || cert == "" || key == "") {
		return makeDockerClient(m.config, endpoint, m.config.Docker.APIVersion)
	}
	if (strings.HasPrefix(endpoint, nodeSockPrefixKey) ||
		strings.HasPrefix(endpoint, nodeTCPPrefixKey)) &&
		m.config.Docker.CertPath != "" &&
		ca != "" && cert != "" && key != "" {
		caFile, err := ioutil.TempFile(m.config.Docker.CertPath, fmt.Sprintf("ca-%s", nodename))
		if err != nil {
			return nil, err
		}
		certFile, err := ioutil.TempFile(m.config.Docker.CertPath, fmt.Sprintf("cert-%s", nodename))
		if err != nil {
			return nil, err
		}
		keyFile, err := ioutil.TempFile(m.config.Docker.CertPath, fmt.Sprintf("key-%s", nodename))
		if err != nil {
			return nil, err
		}
		if err = dumpFromString(caFile, certFile, keyFile, ca, cert, key); err != nil {
			return nil, err
		}
		return makeDockerClientWithTLS(m.config, caFile, certFile, keyFile, endpoint, m.config.Docker.APIVersion)
	}
	return nil, types.ErrNotSupport
}

func (m *Mercury) doAddNode(ctx context.Context, name, endpoint, podname, ca, cert, key string, cpu, share int, memory int64, labels map[string]string) (*types.Node, error) {
	data := map[string]string{}
	// 如果有tls的证书需要保存就保存一下
	if ca != "" && cert != "" && key != "" {
		data[fmt.Sprintf(nodeCaKey, name)] = ca
		data[fmt.Sprintf(nodeCertKey, name)] = cert
		data[fmt.Sprintf(nodeKeyKey, name)] = key
	}

	cpumap := types.CPUMap{}
	for i := 0; i < cpu; i++ {
		cpumap[strconv.Itoa(i)] = share
	}

	node := &types.Node{
		Name:       name,
		Endpoint:   endpoint,
		Podname:    podname,
		CPU:        cpumap,
		MemCap:     memory,
		InitCPU:    cpumap,
		InitMemCap: memory,
		Available:  true,
		Labels:     labels,
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	data[fmt.Sprintf(nodeInfoKey, podname, name)] = string(bytes)
	data[fmt.Sprintf(nodePodKey, name)] = podname

	_, err = m.BatchCreate(ctx, data)
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
func (m *Mercury) doDeleteNode(ctx context.Context, podname, nodename, endpoint string) {
	keys := []string{
		fmt.Sprintf(nodeInfoKey, podname, nodename),
		fmt.Sprintf(nodePodKey, nodename),
		fmt.Sprintf(nodeCaKey, nodename),
		fmt.Sprintf(nodeCertKey, nodename),
		fmt.Sprintf(nodeKeyKey, nodename),
	}

	m.BatchDelete(ctx, keys)
	_cache.Delete(nodename)
	log.Infof("[doDeleteNode] Node (%s, %s, %s) deleted", podname, nodename, endpoint)
}

func (m *Mercury) doGetNode(ctx context.Context, podname, nodename string) (*types.Node, error) {
	key := fmt.Sprintf(nodeInfoKey, podname, nodename)
	ev, err := m.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	node := &types.Node{}
	err = json.Unmarshal(ev.Value, node)
	return node, err
}
