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

//AddPod add a pod
// save it to etcd
// storage path in etcd is `/pod/:podname/info`
func (k *Krypton) AddPod(ctx context.Context, name, favor, desc string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	favor = strings.ToUpper(favor)
	if favor == "" {
		favor = scheduler.MEMORY_PRIOR
	} else if favor != scheduler.MEMORY_PRIOR && favor != scheduler.CPU_PRIOR {
		return nil, types.NewDetailedErr(types.ErrBadFaver,
			fmt.Sprintf("got bad faver: %s", favor))
	}
	pod := &types.Pod{Name: name, Desc: desc, Favor: favor}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}

	_, err = k.etcd.Create(ctx, key, string(bytes))
	return pod, err
}

//GetPod get a pod from etcd
// storage path in etcd is `/pod/:podname/info`
func (k *Krypton) GetPod(ctx context.Context, name string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	resp, err := k.etcd.Get(ctx, key, nil)
	if err != nil {
		return nil, err
	}
	if resp.Node.Dir {
		return nil, types.NewDetailedErr(types.ErrKeyIsNotDir,
			fmt.Sprintf("pod storage path %q in etcd is a directory", key))
	}

	pod := &types.Pod{}
	err = json.Unmarshal([]byte(resp.Node.Value), pod)
	return pod, err
}

//RemovePod if the pod has no nodes left, otherwise return an error
func (k *Krypton) RemovePod(ctx context.Context, podname string) error {
	key := fmt.Sprintf("%s/%s", allPodsKey, podname)

	ns, err := k.GetNodesByPod(ctx, podname)
	if err != nil && !client.IsKeyNotFound(err) {
		return err
	}
	if l := len(ns); l != 0 {
		return types.NewDetailedErr(types.ErrPodHasNodes,
			fmt.Sprintf("pod %s still has %d nodes, delete them first", podname, l))
	}

	_, err = k.etcd.Delete(ctx, key, &client.DeleteOptions{Dir: true, Recursive: true})
	return err
}

//GetAllPods get all pods in etcd
// any error will break and return error immediately
// storage path in etcd is `/pod`
func (k *Krypton) GetAllPods(ctx context.Context) (pods []*types.Pod, err error) {
	resp, err := k.etcd.Get(ctx, allPodsKey, nil)
	if err != nil {
		return pods, err
	}
	if !resp.Node.Dir {
		return nil, types.NewDetailedErr(types.ErrKeyIsNotDir,
			fmt.Sprintf("pod storage path %q in etcd is not a directory", allPodsKey))
	}

	for _, node := range resp.Node.Nodes {
		name := utils.Tail(node.Key)
		p, err := k.GetPod(ctx, name)
		if err != nil {
			return pods, err
		}
		pods = append(pods, p)
	}
	return pods, err
}
