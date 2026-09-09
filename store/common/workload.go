package common

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"path/filepath"
	"slices"

	"github.com/cockroachdb/errors"
	"golang.org/x/sync/errgroup"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

const statusReaders = 32

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
	return s.getWorkload(ctx, ID, true)
}

func (s *Store) GetWorkloads(ctx context.Context, IDs []string) ([]*types.Workload, error) {
	return s.getWorkloads(ctx, IDs, true)
}

func (s *Store) SetWorkloadStatus(ctx context.Context, status *types.StatusMeta, ttl int64) error {
	if status.Appname == "" || status.Entrypoint == "" || status.Nodename == "" {
		return types.ErrInvaildWorkloadStatus
	}
	if ttl < 0 {
		return types.ErrInvaildWorkloadStatusTTL
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
	utils.SentryGo(func() {
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

func (s *Store) getWorkload(ctx context.Context, ID string, withEngine bool) (*types.Workload, error) {
	workloads, err := s.getWorkloads(ctx, []string{ID}, withEngine)
	if err != nil {
		return nil, err
	}
	return workloads[0], nil
}

func (s *Store) getWorkloads(ctx context.Context, IDs []string, withEngine bool) ([]*types.Workload, error) {
	keys := make([]string, 0, len(IDs))
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(WorkloadInfoKey, ID))
	}

	data, err := s.GetMulti(ctx, keys)
	if err != nil {
		if s.NotFound(err) {
			return nil, errors.Join(types.ErrWorkloadNotExists, err)
		}
		return nil, err
	}

	workloads := []*types.Workload{}
	for _, key := range keys {
		workload := &types.Workload{}
		if err := json.Unmarshal([]byte(data[key]), workload); err != nil {
			log.WithFunc("store.common.getWorkloads").Errorf(ctx, err, "failed to unmarshal %+v", key)
			return nil, err
		}
		workloads = append(workloads, workload)
	}

	return s.bindWorkloadsAdditions(ctx, workloads, withEngine)
}

func (s *Store) workloadStatusStream(ctx context.Context, logger *log.Fields, statusKey string, labels map[string]string, ch chan<- *types.WorkloadStatus) error {
	logger.Infof(ctx, "watch on %s", statusKey)
	for event := range s.Watch(ctx, statusKey) {
		_, _, _, ID := ParseStatusKey(event.Key)
		msg := &types.WorkloadStatus{ID: ID, Delete: event.Type != EventPut}
		workload, err := s.getWorkload(ctx, ID, false)
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

	return s.bindWorkloadsAdditions(ctx, workloads, true)
}

func (s *Store) bindWorkloadsAdditions(ctx context.Context, workloads []*types.Workload, withEngine bool) ([]*types.Workload, error) {
	nodenames := map[string]struct{}{}
	groups := map[string][]*types.Workload{}
	logger := log.WithFunc("store.common.bindWorkloadsAdditions")
	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			return nil, err
		}
		// trailing slash keeps the prefix from matching a longer nodename
		prefix := filepath.Join(WorkloadStatusPrefix, appname, entrypoint, workload.Nodename) + "/"
		groups[prefix] = append(groups[prefix], workload)
		nodenames[workload.Nodename] = struct{}{}
	}
	if withEngine {
		ns, err := s.GetNodes(ctx, slices.Collect(maps.Keys(nodenames)))
		if err != nil {
			return nil, err
		}
		nodes := map[string]*types.Node{}
		for _, node := range ns {
			nodes[node.Name] = node
		}
		for _, workload := range workloads {
			node, ok := nodes[workload.Nodename]
			if !ok {
				return nil, types.ErrInvaildWorkloadMeta
			}
			workload.Engine = node.Engine
		}
	}

	bind := func(prefix string, group []*types.Workload) error {
		data, err := s.GetPrefix(ctx, prefix, 0)
		if err != nil {
			return err
		}
		for _, workload := range group {
			value, ok := data[prefix+workload.ID]
			if !ok {
				continue
			}
			status := &types.StatusMeta{}
			if err := json.Unmarshal([]byte(value), status); err != nil {
				logger.Errorf(ctx, err, "unmarshal status of %s, raw: %s", workload.ID, value)
				continue
			}
			workload.StatusMeta = status
		}
		return nil
	}

	reads := errgroup.Group{}
	reads.SetLimit(statusReaders)
	for prefix, group := range groups {
		reads.Go(func() error { return bind(prefix, group) })
	}
	if err := reads.Wait(); err != nil {
		return nil, err
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
