package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/errors"
	clientv3 "go.etcd.io/etcd/client/v3"

	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
)

func (m *Mercury) AddPod(ctx context.Context, name, desc string) (*types.Pod, error) {
	key := fmt.Sprintf(common.PodInfoKey, name)
	pod := &types.Pod{Name: name, Desc: desc}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	resp, err := m.BatchCreate(ctx, map[string]string{key: string(bytes)})
	if err != nil {
		return nil, err
	}
	if !resp.Succeeded {
		return nil, types.ErrTxnConditionFailed
	}
	return pod, nil
}

func (m *Mercury) RemovePod(ctx context.Context, podname string) error {
	key := fmt.Sprintf(common.PodInfoKey, podname)

	ns, err := m.GetNodesByPod(ctx, &types.NodeFilter{Podname: podname, All: true}, false)
	if err != nil {
		return err
	}

	if l := len(ns); l != 0 {
		return errors.Wrapf(types.ErrPodHasNodes, "pod %s still has %d nodes, delete them first", podname, l)
	}

	resp, err := m.Delete(ctx, key)
	if err != nil {
		return err
	}
	if resp.Deleted != 1 {
		return errors.Wrapf(types.ErrPodNotFound, "podname: %s", podname)
	}
	return nil
}

func (m *Mercury) GetPod(ctx context.Context, name string) (*types.Pod, error) {
	key := fmt.Sprintf(common.PodInfoKey, name)

	ev, err := m.GetOne(ctx, key)
	if err != nil {
		return nil, err
	}

	pod := &types.Pod{}
	if err = json.Unmarshal(ev.Value, pod); err != nil {
		return nil, err
	}
	return pod, nil
}

func (m *Mercury) GetAllPods(ctx context.Context) ([]*types.Pod, error) {
	resp, err := m.Get(ctx, fmt.Sprintf(common.PodInfoKey, ""), clientv3.WithPrefix())
	if err != nil {
		return []*types.Pod{}, err
	}

	pods := []*types.Pod{}
	for _, ev := range resp.Kvs {
		pod := &types.Pod{}
		if err := json.Unmarshal(ev.Value, pod); err != nil {
			return pods, err
		}
		pods = append(pods, pod)
	}
	return pods, nil
}
