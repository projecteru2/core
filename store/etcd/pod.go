package etcdstore

import (
	"encoding/json"
	"fmt"

	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

// get a pod from etcd
// storage path in etcd is `/eru-core/pod/:podname/info`
func (k *Krypton) GetPod(name string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	resp, err := k.etcd.Get(context.Background(), key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, fmt.Errorf("Pod storage path %q in etcd is a directory", key)
	}

	pod := &types.Pod{}
	if err := json.Unmarshal([]byte(resp.Node.Value), pod); err != nil {
		return nil, err
	}

	return pod, nil
}

// add a pod
// save it to etcd
// storage path in etcd is `/eru-core/pod/:podname/info`
func (k *Krypton) AddPod(name, desc string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	pod := &types.Pod{Name: name, Desc: desc}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	_, err = k.etcd.Set(context.Background(), key, string(bytes), nil)
	if err != nil {
		return nil, err
	}

	return pod, nil
}

// get all pods in etcd
// any error will break and return error immediately
// storage path in etcd is `/eru-core/pod`
func (k *Krypton) GetAllPods() ([]*types.Pod, error) {
	var (
		pods []*types.Pod
		err  error
	)

	resp, err := k.etcd.Get(context.Background(), allPodsKey, nil)
	if err != nil {
		return pods, err
	}
	if !resp.Node.Dir {
		return nil, fmt.Errorf("Pod storage path %q in etcd is not a directory", allPodsKey)
	}

	for _, node := range resp.Node.Nodes {
		name := utils.Tail(node.Key)
		p, err := k.GetPod(name)
		if err != nil {
			return pods, err
		}
		pods = append(pods, p)
	}
	return pods, err
}
