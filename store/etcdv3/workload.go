package etcdv3

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
)

func (m *Mercury) AddWorkload(ctx context.Context, workload *types.Workload, processing *types.Processing) error {
	return m.doOpsWorkload(ctx, workload, processing, true)
}

func (m *Mercury) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	return m.doOpsWorkload(ctx, workload, nil, false)
}

func (m *Mercury) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	return m.cleanWorkloadData(ctx, workload)
}

func (m *Mercury) GetWorkload(ctx context.Context, ID string) (*types.Workload, error) {
	workloads, err := m.GetWorkloads(ctx, []string{ID})
	if err != nil {
		return nil, err
	}
	return workloads[0], nil
}

func (m *Mercury) GetWorkloads(ctx context.Context, IDs []string) (workloads []*types.Workload, err error) {
	keys := []string{}
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(workloadInfoKey, ID))
	}

	return m.doGetWorkloads(ctx, keys)
}

func (m *Mercury) GetWorkloadStatus(ctx context.Context, ID string) (*types.StatusMeta, error) {
	workload, err := m.GetWorkload(ctx, ID)
	if err != nil {
		return nil, err
	}
	return workload.StatusMeta, nil
}

func (m *Mercury) SetWorkloadStatus(ctx context.Context, status *types.StatusMeta, ttl int64) error {
	if status.Appname == "" || status.Entrypoint == "" || status.Nodename == "" {
		return types.ErrInvaildWorkloadStatus
	}

	data, err := json.Marshal(status)
	if err != nil {
		return err
	}
	statusVal := string(data)
	statusKey := filepath.Join(workloadStatusPrefix, status.Appname, status.Entrypoint, status.Nodename, status.ID)
	workloadKey := fmt.Sprintf(workloadInfoKey, status.ID)
	return m.BindStatus(ctx, workloadKey, statusKey, statusVal, ttl)
}

func (m *Mercury) ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// trailing slash keeps the prefix from matching a longer nodename
	key := filepath.Join(workloadDeployPrefix, appname, entrypoint, nodename) + "/"
	resp, err := m.Get(ctx, key, clientv3.WithPrefix(), clientv3.WithLimit(limit))
	if err != nil {
		return nil, err
	}

	workloads := []*types.Workload{}
	for _, ev := range resp.Kvs {
		workload := &types.Workload{}
		if err := json.Unmarshal(ev.Value, workload); err != nil {
			return nil, err
		}
		if utils.LabelsFilter(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return m.bindWorkloadsAdditions(ctx, workloads)
}

func (m *Mercury) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error) {
	key := fmt.Sprintf(nodeWorkloadsKey, nodename, "")
	resp, err := m.Get(ctx, key, clientv3.WithPrefix())
	if err != nil {
		return nil, err
	}

	workloads := []*types.Workload{}
	for _, ev := range resp.Kvs {
		workload := &types.Workload{}
		if err := json.Unmarshal(ev.Value, workload); err != nil {
			return nil, err
		}
		if utils.LabelsFilter(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return m.bindWorkloadsAdditions(ctx, workloads)
}

func (m *Mercury) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// trailing slash keeps the prefix from matching a longer nodename
	statusKey := filepath.Join(workloadStatusPrefix, appname, entrypoint, nodename) + "/"
	ch := make(chan *types.WorkloadStatus)
	logger := log.WithFunc("store.etcdv3.WorkloadStatusStream")
	_ = m.pool.Invoke(func() {
		defer func() {
			logger.Info(ctx, "close WorkloadStatus channel")
			close(ch)
		}()

		logger.Infof(ctx, "watch on %s", statusKey)
		for resp := range m.Watch(ctx, statusKey, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				if !resp.Canceled {
					logger.Error(ctx, resp.Err(), "watch failed")
				}
				return
			}
			for _, ev := range resp.Events {
				_, _, _, ID := parseStatusKey(string(ev.Kv.Key))
				msg := &types.WorkloadStatus{ID: ID, Delete: ev.Type == clientv3.EventTypeDelete}
				workload, err := m.GetWorkload(ctx, ID)
				switch {
				case err != nil:
					msg.Error = err
				case utils.LabelsFilter(workload.Labels, labels):
					logger.Debugf(ctx, "workload %s status changed", workload.ID)
					msg.Workload = workload
				default:
					continue
				}
				ch <- msg
			}
		}
	})
	return ch
}

func (m *Mercury) cleanWorkloadData(ctx context.Context, workload *types.Workload) error {
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}

	keys := []string{
		filepath.Join(workloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID),
		filepath.Join(workloadDeployPrefix, appname, entrypoint, workload.Nodename, workload.ID),
		fmt.Sprintf(workloadInfoKey, workload.ID),
		fmt.Sprintf(nodeWorkloadsKey, workload.Nodename, workload.ID),
	}
	_, err = m.BatchDelete(ctx, keys)
	return err
}

func (m *Mercury) doGetWorkloads(ctx context.Context, keys []string) (workloads []*types.Workload, err error) {
	var kvs []*mvccpb.KeyValue
	if kvs, err = m.GetMulti(ctx, keys); err != nil {
		return workloads, err
	}

	for _, kv := range kvs {
		workload := &types.Workload{}
		if err = json.Unmarshal(kv.Value, workload); err != nil {
			log.WithFunc("store.etcdv3.doGetWorkloads").Errorf(ctx, err, "failed to unmarshal %+v", string(kv.Key))
			return workloads, err
		}
		workloads = append(workloads, workload)
	}

	return m.bindWorkloadsAdditions(ctx, workloads)
}

func (m *Mercury) bindWorkloadsAdditions(ctx context.Context, workloads []*types.Workload) ([]*types.Workload, error) {
	nodes := map[string]*types.Node{}
	nodenames := []string{}
	nodenameCache := map[string]struct{}{}
	statusKeys := map[string]string{}
	logger := log.WithFunc("store.etcdv3.bindWorkloadsAdditions")
	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			return nil, err
		}
		statusKeys[workload.ID] = filepath.Join(workloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID)
		if _, ok := nodenameCache[workload.Nodename]; !ok {
			nodenameCache[workload.Nodename] = struct{}{}
			nodenames = append(nodenames, workload.Nodename)
		}
	}
	ns, err := m.GetNodes(ctx, nodenames)
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
		kv, err := m.GetOne(ctx, statusKeys[workload.ID])
		if err != nil {
			continue
		}
		status := &types.StatusMeta{}
		if err := json.Unmarshal(kv.Value, &status); err != nil {
			logger.Errorf(ctx, err, "unmarshal status of %s, raw: %s", workload.ID, kv.Value)
			continue
		}
		workloads[index].StatusMeta = status
	}
	return workloads, nil
}

func (m *Mercury) doOpsWorkload(ctx context.Context, workload *types.Workload, processing *types.Processing, create bool) error {
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
		fmt.Sprintf(workloadInfoKey, workload.ID):                                                workloadData,
		fmt.Sprintf(nodeWorkloadsKey, workload.Nodename, workload.ID):                            workloadData,
		filepath.Join(workloadDeployPrefix, appname, entrypoint, workload.Nodename, workload.ID): workloadData,
	}

	var resp *clientv3.TxnResponse
	if create {
		if processing != nil {
			processingKey := m.getProcessingKey(processing)
			err = m.BatchCreateAndDecr(ctx, data, processingKey)
		} else {
			resp, err = m.BatchCreate(ctx, data)
		}
	} else {
		resp, err = m.BatchUpdate(ctx, data)
	}
	if err != nil {
		return err
	}
	if resp != nil && !resp.Succeeded {
		return types.ErrTxnConditionFailed
	}
	return nil
}
