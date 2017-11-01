package etcdstore

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/coreos/etcd/client"
	"github.com/projecteru2/core/scheduler"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// get a pod from etcd
// storage path in etcd is `/pod/:podname/info`
func (k *krypton) GetPod(name string) (*types.Pod, error) {
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
// storage path in etcd is `/pod/:podname/info`
func (k *krypton) AddPod(name, favor, desc string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	favor = strings.ToUpper(favor)
	if favor == "" {
		favor = scheduler.MEMORY_PRIOR
	} else if favor != scheduler.MEMORY_PRIOR && favor != scheduler.CPU_PRIOR {
		return nil, fmt.Errorf("favor should be either CPU or MEM, got %s", favor)
	}
	pod := &types.Pod{Name: name, Desc: desc, Favor: favor}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	_, err = k.etcd.Create(context.Background(), key, string(bytes))
	if err != nil {
		return nil, err
	}

	return pod, nil
}

// get all pods in etcd
// any error will break and return error immediately
// storage path in etcd is `/pod`
func (k *krypton) GetAllPods() (pods []*types.Pod, err error) {
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

// RemovePod if the pod has no nodes left, otherwise return an error
func (k *krypton) RemovePod(podname string) error {
	key := fmt.Sprintf("%s/%s", allPodsKey, podname)

	ns, err := k.GetNodesByPod(podname)
	if err != nil && !client.IsKeyNotFound(err) {
		return err
	}
	if len(ns) != 0 {
		return fmt.Errorf("Pod %s still has %d nodes, delete them first", podname, len(ns))
	}

	_, err = k.etcd.Delete(context.Background(), key, &client.DeleteOptions{Dir: true, Recursive: true})
	return err
}
