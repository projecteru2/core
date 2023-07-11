package redis

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/errors"
	"github.com/projecteru2/core/types"
)

// AddPod adds a pod to core
func (r *Rediaron) AddPod(ctx context.Context, name, desc string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)
	pod := &types.Pod{Name: name, Desc: desc}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	err = r.BatchCreate(ctx, map[string]string{key: string(bytes)})
	return pod, err
}

// RemovePod removes a pod by name
func (r *Rediaron) RemovePod(ctx context.Context, podname string) error {
	key := fmt.Sprintf(podInfoKey, podname)

	ns, err := r.GetNodesByPod(ctx, &types.NodeFilter{Podname: podname, All: true})
	if err != nil {
		return err
	}

	if l := len(ns); l != 0 {
		return errors.Wrapf(types.ErrPodHasNodes, "pod %s still has %d nodes, delete them first", podname, l)
	}

	_, err = r.cli.Del(ctx, key).Result()
	return err
}

// GetPod gets a pod by name
func (r *Rediaron) GetPod(ctx context.Context, name string) (*types.Pod, error) {
	key := fmt.Sprintf(podInfoKey, name)

	data, err := r.cli.Get(ctx, key).Result()
	if err != nil {
		return nil, err
	}

	pod := &types.Pod{}
	if err = json.Unmarshal([]byte(data), pod); err != nil {
		return nil, err
	}
	return pod, err
}

// GetAllPods list all pods in core
func (r *Rediaron) GetAllPods(ctx context.Context) ([]*types.Pod, error) {
	data, err := r.getByKeyPattern(ctx, fmt.Sprintf(podInfoKey, "*"), 0)
	if err != nil {
		return nil, err
	}

	pods := []*types.Pod{}
	for _, value := range data {
		pod := &types.Pod{}
		if err := json.Unmarshal([]byte(value), pod); err != nil {
			return nil, err
		}
		pods = append(pods, pod)
	}
	return pods, nil
}
