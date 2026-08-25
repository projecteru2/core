package common

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/types"
)

func (s *Store) AddPod(ctx context.Context, name, desc string) (*types.Pod, error) {
	pod := &types.Pod{Name: name, Desc: desc}

	bytes, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	if err := s.Create(ctx, map[string]string{fmt.Sprintf(PodInfoKey, name): string(bytes)}); err != nil {
		return nil, err
	}
	return pod, nil
}

func (s *Store) RemovePod(ctx context.Context, podname string) error {
	ns, err := s.GetNodesByPod(ctx, &types.NodeFilter{Podname: podname, All: true}, false)
	if err != nil {
		return err
	}
	if l := len(ns); l != 0 {
		return errors.Wrapf(types.ErrPodHasNodes, "pod %s still has %d nodes, delete them first", podname, l)
	}

	key := fmt.Sprintf(PodInfoKey, podname)
	if _, err := s.GetOne(ctx, key); err != nil {
		if s.NotFound(err) {
			return errors.Wrapf(types.ErrPodNotFound, "podname: %s", podname)
		}
		return err
	}
	return s.Delete(ctx, []string{key})
}

func (s *Store) GetPod(ctx context.Context, name string) (*types.Pod, error) {
	value, err := s.GetOne(ctx, fmt.Sprintf(PodInfoKey, name))
	if err != nil {
		return nil, err
	}

	pod := &types.Pod{}
	if err := json.Unmarshal([]byte(value), pod); err != nil {
		return nil, err
	}
	return pod, nil
}

func (s *Store) GetAllPods(ctx context.Context) ([]*types.Pod, error) {
	data, err := s.GetPrefix(ctx, fmt.Sprintf(PodInfoKey, ""), 0)
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
