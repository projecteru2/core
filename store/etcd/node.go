package etcdstore

import (
	"encoding/json"
	"fmt"
	"strconv"

	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

// get a node from etcd
// and construct it's docker client
// a node must belong to a pod
// and since node is not the smallest unit to user, to get a node we must specify the corresponding pod
// storage path in etcd is `/eru-core/pod/:podname/node/:nodename/info`
func (k *Krypton) GetNode(podname, nodename string) (*types.Node, error) {
	key := fmt.Sprintf(nodeInfoKey, podname, nodename)
	resp, err := k.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, fmt.Errorf("Node storage path %q in etcd is a directory", key)
	}

	node := &Node{}
	if err := json.Unmarshal([]byte(resp.Node.Value), node); err != nil {
		return nil, err
	}

	engine, err := utils.MakeDockerClient(node.Endpoint, k.config)
	if err != nil {
		return nil, err
	}

	node.Engine = engine
	return node, nil
}

// add a node
// save it to etcd
// storage path in etcd is `/eru-core/pod/:podname/node/:nodename/info`
func (k *Krypton) AddNode(name, endpoint, podname string, public bool) (*types.Node, error) {
	engine, err := utils.MakeDockerClient(endpoint, k.config)
	if err != nil {
		return nil, err
	}

	info, err := engine.Info(context.Background())
	if err != nil {
		return nil, err
	}

	cores := make(map[string]int)
	for i := 0; i < info.NCPU; i++ {
		cores[strconv.itoa(i)] = 1
	}

	key := fmt.Sprintf(podNodesKey, podname, name)
	node := &types.Node{
		Name:     name,
		Endpoint: endpoint,
		Podname:  podname,
		Public:   public,
		Cores:    cores,
		Engine:   engine,
	}

	bytes, err := json.Marshal(node)
	if err != nil {
		return nil, err
	}

	_, err = k.etcd.Set(context.Background(), key, string(bytes), nil)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// get all nodes from etcd
// any error will break and return immediately
func (k *Krypton) GetAllNodes() ([]*types.Node, error) {
	var (
		nodes []*types.Node
		err   error
	)

	pods, err := k.GetAllPods()
	if err != nil {
		return nodes, err
	}

	for _, pod := range pods {
		ns, err := k.GetNodesByPod(pod.Name)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, ns...)
	}
	return nodes, err
}

// get all nodes bound to pod
// here we use podname instead of pod instance
// storage path in etcd is `/eru-core/pod/:podname/node`
func (k *Krypton) GetNodesByPod(podname string) ([]*types.Node, error) {
	var (
		nodes []*types.Node
		err   error
	)

	key := fmt.Sprintf(podNodesKey, podname)
	resp, err := k.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nodes, err
	}
	if !resp.Node.Dir {
		return nil, fmt.Errorf("Node storage path %q in etcd is not a directory", key)
	}

	for _, node := range resp.Node.Nodes {
		nodename := utils.Tail(node.Key)
		n, err := k.GetNode(podname, nodename)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, err
}

// update a node, save it to etcd
// storage path in etcd is `/eru-core/pod/:podname/node/:nodename/info`
func (k *Krypton) UpdateNode(node *types.Node) error {
	key := fmt.Sprintf(podNodesKey, podname, name)
	bytes, err := json.Marshal(node)
	if err != nil {
		return err
	}

	_, err = k.etcd.Set(context.Background(), key, string(bytes), nil)
	if err != nil {
		return err
	}

	return nil
}
