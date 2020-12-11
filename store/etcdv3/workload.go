package etcdv3

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	"go.etcd.io/etcd/clientv3"
	"go.etcd.io/etcd/mvcc/mvccpb"
)

// AddWorkload add a workload
// mainly record its relationship on pod and node
// actually if we already know its node, we will know its pod
// but we still store it
// storage path in etcd is `/workload/:workloadid`
func (m *Mercury) AddWorkload(ctx context.Context, workload *types.Workload) error {
	return m.doOpsWorkload(ctx, workload, true)
}

// UpdateWorkload update a workload
func (m *Mercury) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	return m.doOpsWorkload(ctx, workload, false)
}

// RemoveWorkload remove a workload
// workload id must be in full length
func (m *Mercury) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	return m.cleanWorkloadData(ctx, workload)
}

// GetWorkload get a workload
// workload if must be in full length, or we can't find it in etcd
// storage path in etcd is `/workload/:workloadid`
func (m *Mercury) GetWorkload(ctx context.Context, ID string) (*types.Workload, error) {
	workloads, err := m.GetWorkloads(ctx, []string{ID})
	if err != nil {
		return nil, err
	}
	return workloads[0], nil
}

// GetWorkloads get many workloads
func (m *Mercury) GetWorkloads(ctx context.Context, IDs []string) (workloads []*types.Workload, err error) {
	keys := []string{}
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(workloadInfoKey, ID))
	}

	return m.doGetWorkloads(ctx, keys)
}

// GetWorkloadStatus get workload status
func (m *Mercury) GetWorkloadStatus(ctx context.Context, ID string) (*types.StatusMeta, error) {
	workload, err := m.GetWorkload(ctx, ID)
	if err != nil {
		return nil, err
	}
	return workload.StatusMeta, nil
}

// SetWorkloadStatus set workload status
func (m *Mercury) SetWorkloadStatus(ctx context.Context, workload *types.Workload, ttl int64) error {
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(workload.StatusMeta)
	if err != nil {
		return err
	}
	val := string(data)
	statusKey := filepath.Join(workloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID)
	updateStatus := []clientv3.Op{clientv3.OpPut(statusKey, val)}
	lease := &clientv3.LeaseGrantResponse{}
	if ttl != 0 {
		if lease, err = m.cliv3.Grant(ctx, ttl); err != nil {
			return err
		}
		updateStatus = []clientv3.Op{clientv3.OpPut(statusKey, val, clientv3.WithLease(lease.ID))}
	}
	tr, err := m.cliv3.Txn(ctx).
		If(clientv3.Compare(clientv3.Version(fmt.Sprintf(workloadInfoKey, workload.ID)), "!=", 0)).
		Then( // 保证有容器
			clientv3.OpTxn(
				[]clientv3.Cmp{clientv3.Compare(clientv3.Version(statusKey), "!=", 0)}, // 判断是否有 status key
				[]clientv3.Op{clientv3.OpTxn( // 有 status key
					[]clientv3.Cmp{clientv3.Compare(clientv3.Value(statusKey), "=", val)},
					[]clientv3.Op{clientv3.OpGet(statusKey)}, // status 没修改，返回 status
					updateStatus,                             // 内容修改了就换一个 lease
				)},
				updateStatus, // 没有 status key
			),
		).Commit()
	if err != nil {
		return err
	}
	if !tr.Succeeded { // 没容器了退出
		return nil
	}
	tr2 := tr.Responses[0].GetResponseTxn()
	if !tr2.Succeeded { // 没 status key 直接 put
		lease.ID = 0
		return nil
	}
	tr3 := tr2.Responses[0].GetResponseTxn()
	if tr3.Succeeded && ttl != 0 { // 有 status 并且内容还跟之前一样
		oldLeaseID := clientv3.LeaseID(tr3.Responses[0].GetResponseRange().Kvs[0].Lease) // 拿到 status 绑定的 leaseID
		_, err := m.cliv3.KeepAliveOnce(ctx, oldLeaseID)                                 // 刷新 lease
		return err
	}
	return nil
}

// ListWorkloads list workloads
func (m *Mercury) ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 这里显式加个 / 来保证 prefix 是唯一的
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
		if utils.FilterWorkload(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return m.bindWorkloadsAdditions(ctx, workloads)
}

// ListNodeWorkloads list workloads belong to one node
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
		if utils.FilterWorkload(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return m.bindWorkloadsAdditions(ctx, workloads)
}

// WorkloadStatusStream watch deployed status
func (m *Mercury) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 显式加个 / 保证 prefix 唯一
	statusKey := filepath.Join(workloadStatusPrefix, appname, entrypoint, nodename) + "/"
	ch := make(chan *types.WorkloadStatus)
	go func() {
		defer func() {
			log.Info("[WorkloadStatusStream] close WorkloadStatus channel")
			close(ch)
		}()

		log.Infof("[WorkloadStatusStream] watch on %s", statusKey)
		for resp := range m.watch(ctx, statusKey, clientv3.WithPrefix()) {
			if resp.Err() != nil {
				if !resp.Canceled {
					log.Errorf("[WorkloadStatusStream] watch failed %v", resp.Err())
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
				case utils.FilterWorkload(workload.Labels, labels):
					log.Debugf("[WorkloadStatusStream] workload %s status changed", workload.ID)
					msg.Workload = workload
				default:
					continue
				}
				ch <- msg
			}
		}
	}()
	return ch
}

func (m *Mercury) cleanWorkloadData(ctx context.Context, workload *types.Workload) error {
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}

	keys := []string{
		filepath.Join(workloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID), // workload deploy status
		filepath.Join(workloadDeployPrefix, appname, entrypoint, workload.Nodename, workload.ID), // workload deploy status
		fmt.Sprintf(workloadInfoKey, workload.ID),                                                // workload info
		fmt.Sprintf(nodeWorkloadsKey, workload.Nodename, workload.ID),                            // node workloads
	}
	_, err = m.batchDelete(ctx, keys)
	return err
}

func (m *Mercury) doGetWorkloads(ctx context.Context, keys []string) (workloads []*types.Workload, err error) {
	var kvs []*mvccpb.KeyValue
	if kvs, err = m.GetMulti(ctx, keys); err != nil {
		return
	}

	for _, kv := range kvs {
		workload := &types.Workload{}
		if err = json.Unmarshal(kv.Value, workload); err != nil {
			log.Errorf("[doGetWorkloads] failed to unmarshal %v, err: %v", string(kv.Key), err)
			return
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
			return nil, types.ErrBadMeta
		}
		workloads[index].Engine = nodes[workload.Nodename].Engine
		if _, ok := statusKeys[workload.ID]; !ok {
			continue
		}
		kv, err := m.GetOne(ctx, statusKeys[workload.ID])
		if err != nil {
			continue
		}
		status := &types.StatusMeta{}
		if err := json.Unmarshal(kv.Value, &status); err != nil {
			log.Warnf("[bindWorkloadsAdditions] unmarshal %s status data failed %v", workload.ID, err)
			log.Errorf("[bindWorkloadsAdditions] status raw: %s", kv.Value)
			continue
		}
		workloads[index].StatusMeta = status
	}
	return workloads, nil
}

func (m *Mercury) doOpsWorkload(ctx context.Context, workload *types.Workload, create bool) error {
	var err error
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}

	// now everything is ok
	// we use full length id instead
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

	if create {
		_, err = m.batchCreate(ctx, data)
	} else {
		_, err = m.batchUpdate(ctx, data)
	}
	return err
}
