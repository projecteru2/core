package common

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (s *Store) AddWorkload(ctx context.Context, workload *types.Workload, processing *types.Processing) error {
	return s.doOpsWorkload(ctx, workload, processing, true)
}

func (s *Store) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	return s.doOpsWorkload(ctx, workload, nil, false)
}

func (s *Store) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}

	return s.Delete(ctx, []string{
		filepath.Join(WorkloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID),
		filepath.Join(WorkloadDeployPrefix, appname, entrypoint, workload.Nodename, workload.ID),
		fmt.Sprintf(WorkloadInfoKey, workload.ID),
		fmt.Sprintf(NodeWorkloadsKey, workload.Nodename, workload.ID),
	})
}

func (s *Store) GetWorkload(ctx context.Context, ID string) (*types.Workload, error) {
	workloads, err := s.GetWorkloads(ctx, []string{ID})
	if err != nil {
		return nil, err
	}
	return workloads[0], nil
}

func (s *Store) GetWorkloads(ctx context.Context, IDs []string) ([]*types.Workload, error) {
	keys := make([]string, 0, len(IDs))
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(WorkloadInfoKey, ID))
	}

	data, err := s.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	workloads := []*types.Workload{}
	for _, key := range keys {
		workload := &types.Workload{}
		if err := json.Unmarshal([]byte(data[key]), workload); err != nil {
			log.WithFunc("store.common.GetWorkloads").Errorf(ctx, err, "failed to unmarshal %+v", key)
			return nil, err
		}
		workloads = append(workloads, workload)
	}

	return s.bindWorkloadsAdditions(ctx, workloads)
}

func (s *Store) GetWorkloadStatus(ctx context.Context, ID string) (*types.StatusMeta, error) {
	workload, err := s.GetWorkload(ctx, ID)
	if err != nil {
		return nil, err
	}
	return workload.StatusMeta, nil
}

func (s *Store) SetWorkloadStatus(ctx context.Context, status *types.StatusMeta, ttl int64) error {
	if status.Appname == "" || status.Entrypoint == "" || status.Nodename == "" {
		return types.ErrInvaildWorkloadStatus
	}

	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	statusKey := filepath.Join(WorkloadStatusPrefix, status.Appname, status.Entrypoint, status.Nodename, status.ID)
	workloadKey := fmt.Sprintf(WorkloadInfoKey, status.ID)
	return s.BindStatus(ctx, workloadKey, statusKey, string(data), ttl)
}

func (s *Store) ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// trailing slash keeps the prefix from matching a longer nodename
	data, err := s.GetPrefix(ctx, filepath.Join(WorkloadDeployPrefix, appname, entrypoint, nodename)+"/", limit)
	if err != nil {
		return nil, err
	}
	return s.filterWorkloads(ctx, data, labels)
}

func (s *Store) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error) {
	data, err := s.GetPrefix(ctx, fmt.Sprintf(NodeWorkloadsKey, nodename, ""), 0)
	if err != nil {
		return nil, err
	}
	return s.filterWorkloads(ctx, data, labels)
}

func (s *Store) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// trailing slash keeps the prefix from matching a longer nodename
	statusKey := filepath.Join(WorkloadStatusPrefix, appname, entrypoint, nodename) + "/"
	ch := make(chan *types.WorkloadStatus)
	logger := log.WithFunc("store.common.WorkloadStatusStream")
	_ = s.Pool.Invoke(func() {
		defer func() {
			logger.Info(ctx, "close WorkloadStatus channel")
			close(ch)
		}()
		if err := s.workloadStatusStream(ctx, logger, statusKey, labels, ch); err != nil && ctx.Err() == nil {
			logger.Error(ctx, err, "workload status stream interrupted")
		}
	})
	return ch
}

func (s *Store) workloadStatusStream(ctx context.Context, logger *log.Fields, statusKey string, labels map[string]string, ch chan<- *types.WorkloadStatus) error {
	logger.Infof(ctx, "watch on %s", statusKey)
	for event := range s.Watch(ctx, statusKey) {
		_, _, _, ID := ParseStatusKey(event.Key)
		msg := &types.WorkloadStatus{ID: ID, Delete: event.Type != EventPut}
		workload, err := s.GetWorkload(ctx, ID)
		switch {
		case err != nil:
			msg.Error = err
		case utils.LabelsFilter(workload.Labels, labels):
			logger.Debugf(ctx, "workload %s status changed", workload.ID)
			msg.Workload = workload
		default:
			continue
		}
		select {
		case ch <- msg:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return types.ErrMessageChanClosed
}

func (s *Store) filterWorkloads(ctx context.Context, data, labels map[string]string) ([]*types.Workload, error) {
	workloads := []*types.Workload{}
	for _, key := range slices.Sorted(maps.Keys(data)) {
		workload := &types.Workload{}
		if err := json.Unmarshal([]byte(data[key]), workload); err != nil {
			return nil, err
		}
		if utils.LabelsFilter(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return s.bindWorkloadsAdditions(ctx, workloads)
}

func (s *Store) bindWorkloadsAdditions(ctx context.Context, workloads []*types.Workload) ([]*types.Workload, error) {
	nodes := map[string]*types.Node{}
	nodenames := []string{}
	nodenameCache := map[string]struct{}{}
	statusKeys := map[string]string{}
	logger := log.WithFunc("store.common.bindWorkloadsAdditions")
	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			return nil, err
		}
		statusKeys[workload.ID] = filepath.Join(WorkloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID)
		if _, ok := nodenameCache[workload.Nodename]; !ok {
			nodenameCache[workload.Nodename] = struct{}{}
			nodenames = append(nodenames, workload.Nodename)
		}
	}
	ns, err := s.GetNodes(ctx, nodenames)
	if err != nil {
		return nil, err
	}
	for _, node := range ns {
		nodes[node.Name] = node
	}

	for index, workload := range workloads {
		if _, ok := nodes[workload.Nodename]; !ok {
			return nil, types.ErrInvaildWorkloadMeta
		}
		workloads[index].Engine = nodes[workload.Nodename].Engine
		value, err := s.GetOne(ctx, statusKeys[workload.ID])
		if err != nil {
			continue
		}
		status := &types.StatusMeta{}
		if err := json.Unmarshal([]byte(value), &status); err != nil {
			logger.Errorf(ctx, err, "unmarshal status of %s, raw: %s", workload.ID, value)
			continue
		}
		workloads[index].StatusMeta = status
	}
	return workloads, nil
}

func (s *Store) doOpsWorkload(ctx context.Context, workload *types.Workload, processing *types.Processing, create bool) error {
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}

	bytes, err := json.Marshal(workload)
	if err != nil {
		return err
	}
	workloadData := string(bytes)

	data := map[string]string{
		fmt.Sprintf(WorkloadInfoKey, workload.ID):                                                workloadData,
		fmt.Sprintf(NodeWorkloadsKey, workload.Nodename, workload.ID):                            workloadData,
		filepath.Join(WorkloadDeployPrefix, appname, entrypoint, workload.Nodename, workload.ID): workloadData,
	}

	switch {
	case !create:
		return s.Update(ctx, data)
	case processing != nil:
		return s.CreateAndDecr(ctx, data, ProcessingKey(processing))
	default:
		return s.Create(ctx, data)
	}
}
