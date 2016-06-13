package store

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/coreos/etcd/client"
	"gitlab.ricebook.net/core/types"
	"gitlab.ricebook.net/core/utils"
	"golang.org/x/net/context"
)

var (
	allPodsKey       = "/eru-core/pod"
	podInfoKey       = "/eru-core/pod/%s/info"
	podNodesKey      = "/eru-core/pod/%s/node"
	nodeInfoKey      = "/eru-core/pod/%s/node/%s/info"
	nodeContainerKey = "/eru-core/pod/%s/node/%s/containers"
)

type Store struct {
	etcd   *client.KeysAPI
	config *types.Config
}

// get a pod from etcd
func (s *Store) GetPod(name string) (types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	resp, err := s.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, fmt.Errorf("Pod storage path %q in etcd is a directory", key)
	}

	pod := Pod{}
	if err := json.Unmarshal([]byte(resp.Node.Value), &pod); err != nil {
		return nil, err
	}

	return pod, nil
}

// add a pod
// save it to etcd
func (s *Store) AddPod(name, desc string) (types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	pod := types.Pod{Name: name, Desc: desc}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	_, err = s.etcd.Set(context.Background(), key, string(bytes).nil)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

// get all pods in etcd
// any error will break and return error immediately
func (s *Store) GetAllPods() ([]types.Pod, error) {
	var (
		pods []types.Pod
		err  error
	)

	resp, err := s.etcd.Get(context.Background(), allPodsKey, nil)
	if err != nil {
		return pods, err
	}
	if !resp.Node.Dir {
		return nil, fmt.Errorf("Pod storage path %q in etcd is not a directory", allPodsKey)
	}

	for _, node := range resp.Node.Nodes {
		name := utils.Tail(node.Key)
		p, err := s.GetPod(name)
		if err != nil {
			return pods, err
		}
		pods = append(pods, p)
	}
	return pods, err
}

// get a node from etcd
// and construct it's docker client
// a node must belong to a pod
// and since node is not the smallest unit to user, to get a node we must specify the corresponding pod
func (s *Store) GetNode(podname, nodename string) (types.Node, error) {
	key := fmt.Sprintf(nodeInfoKey, podname, nodename)
	resp, err := s.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, fmt.Errorf("Node storage path %q in etcd is a directory", key)
	}

	node := Node{}
	if err := json.Unmarshal([]byte(resp.Node.Value), &node); err != nil {
		return nil, err
	}

	engine, err := utils.MakeDockerClient(node.Endpoint, s.config)
	if err != nil {
		return nil, err
	}

	node.Engine = engine
	return node, nil
}

// add a node
// save it to etcd
func (s *Store) AddNode(name, endpoint, podname string, public bool) (types.Node, error) {
	engine, err := utils.MakeDockerClient(endpoint, s.config)
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
	node := types.Node{
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

	_, err = s.etcd.Set(context.Background(), key, string(bytes).nil)
	if err != nil {
		return nil, err
	}

	return node, nil
}

// get all nodes from etcd
// any error will break and return immediately
func (s *Store) GetAllNodes() ([]types.Node, error) {
	var (
		nodes []types.Node
		err   error
	)

	pods, err := s.GetAllPods()
	if err != nil {
		return nodes, err
	}

	for _, pod := range pods {
		ns, err := s.GetNodesByPod(pod.Name)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, ns...)
	}
	return nodes, err
}

// get all nodes bound to pod
// here we use podname instead of pod instance
func (s *Store) GetNodesByPod(podname string) ([]types.Node, error) {
	var (
		nodes []types.Node
		err   error
	)

	key := fmt.Sprintf(podNodesKey, podname)
	resp, err := s.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nodes, err
	}
	if !resp.Node.Dir {
		return nil, fmt.Errorf("Node storage path %q in etcd is not a directory", key)
	}

	for _, node := range resp.Node.Nodes {
		nodename := utils.Tail(node.Key)
		n, err := s.GetNode(podname, nodename)
		if err != nil {
			return nodes, err
		}
		nodes = append(nodes, n)
	}
	return nodes, err
}

func NewStore(config *types.Config) (*Store, error) {
	if len(config.EtcdMachines) == 0 {
		return nil, fmt.Errorf("Must set ETCD")
	}

	cli, err := client.New(client.Config{Endpoints: config.EtcdMachines})
	if err != nil {
		return nil, err
	}

	etcd := &client.NewKeysAPI(cli)
	return &Store{etcd: etcd, config: config}, nil
}
