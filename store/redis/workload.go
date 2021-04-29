package redis

import (
	"encoding/json"
	"fmt"
	"path/filepath"

	"context"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// AddWorkload add a workload
// mainly record its relationship on pod and node
// actually if we already know its node, we will know its pod
// but we still store it
// storage path in etcd is `/workload/:workloadid`
func (r *Rediaron) AddWorkload(ctx context.Context, workload *types.Workload) error {
	return r.doOpsWorkload(ctx, workload, true)
}

// UpdateWorkload update a workload
func (r *Rediaron) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	return r.doOpsWorkload(ctx, workload, false)
}

// RemoveWorkload remove a workload
// workload id must be in full length
func (r *Rediaron) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	return r.cleanWorkloadData(ctx, workload)
}

// GetWorkload get a workload
// workload if must be in full length, or we can't find it in etcd
// storage path in etcd is `/workload/:workloadid`
func (r *Rediaron) GetWorkload(ctx context.Context, id string) (*types.Workload, error) {
	workloads, err := r.GetWorkloads(ctx, []string{id})
	if err != nil {
		return nil, err
	}
	return workloads[0], nil
}

// GetWorkloads get many workloads
func (r *Rediaron) GetWorkloads(ctx context.Context, ids []string) (workloads []*types.Workload, err error) {
	keys := []string{}
	for _, id := range ids {
		keys = append(keys, fmt.Sprintf(workloadInfoKey, id))
	}

	return r.doGetWorkloads(ctx, keys)
}

// GetWorkloadStatus get workload status
func (r *Rediaron) GetWorkloadStatus(ctx context.Context, id string) (*types.StatusMeta, error) {
	workload, err := r.GetWorkload(ctx, id)
	if err != nil {
		return nil, err
	}
	return workload.StatusMeta, nil
}

// SetWorkloadStatus set workload status
func (r *Rediaron) SetWorkloadStatus(ctx context.Context, workload *types.Workload, ttl int64) error {
	appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
	if err != nil {
		return err
	}
	data, err := json.Marshal(workload.StatusMeta)
	if err != nil {
		return err
	}
	statusVal := string(data)
	statusKey := filepath.Join(workloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID)
	workloadKey := fmt.Sprintf(workloadInfoKey, workload.ID)
	return r.BindStatus(ctx, workloadKey, statusKey, statusVal, ttl)
}

// ListWorkloads list workloads
func (r *Rediaron) ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 这里显式加个 / 来保证 prefix 是唯一的
	key := filepath.Join(workloadDeployPrefix, appname, entrypoint, nodename) + "/*"
	data, err := r.getByKeyPattern(ctx, key, limit)
	if err != nil {
		return nil, err
	}

	workloads := []*types.Workload{}
	for _, v := range data {
		workload := &types.Workload{}
		if err := json.Unmarshal([]byte(v), workload); err != nil {
			return nil, err
		}
		if utils.FilterWorkload(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return r.bindWorkloadsAdditions(ctx, workloads)
}

// ListNodeWorkloads list workloads belong to one node
func (r *Rediaron) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error) {
	key := fmt.Sprintf(nodeWorkloadsKey, nodename, "*")
	data, err := r.getByKeyPattern(ctx, key, 0)
	if err != nil {
		return nil, err
	}

	workloads := []*types.Workload{}
	for _, v := range data {
		workload := &types.Workload{}
		if err := json.Unmarshal([]byte(v), workload); err != nil {
			return nil, err
		}
		if utils.FilterWorkload(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return r.bindWorkloadsAdditions(ctx, workloads)
}

// WorkloadStatusStream watch deployed status
func (r *Rediaron) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// 显式加个 / 保证 prefix 唯一
	statusKey := filepath.Join(workloadStatusPrefix, appname, entrypoint, nodename) + "/*"
	ch := make(chan *types.WorkloadStatus)
	go func() {
		defer func() {
			log.Info("[WorkloadStatusStream] close WorkloadStatus channel")
			close(ch)
		}()

		log.Infof(ctx, "[WorkloadStatusStream] watch on %s", statusKey)
		for message := range r.KNotify(ctx, statusKey) {
			_, _, _, id := parseStatusKey(message.Key)
			msg := &types.WorkloadStatus{
				ID:     id,
				Delete: message.Action == actionDel || message.Action == actionExpired,
			}
			workload, err := r.GetWorkload(ctx, id)
			switch {
			case err != nil:
				msg.Error = err
			case utils.FilterWorkload(workload.Labels, labels):
				log.Debugf(ctx, "[WorkloadStatusStream] workload %s status changed", workload.ID)
				msg.Workload = workload
			default:
				continue
			}
			ch <- msg
		}
	}()
	return ch
}

func (r *Rediaron) cleanWorkloadData(ctx context.Context, workload *types.Workload) error {
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
	return r.BatchDelete(ctx, keys)
}

func (r *Rediaron) doGetWorkloads(ctx context.Context, keys []string) ([]*types.Workload, error) {
	data, err := r.GetMulti(ctx, keys)
	if err != nil {
		return nil, err
	}

	workloads := []*types.Workload{}
	for k, v := range data {
		workload := &types.Workload{}
		if err = json.Unmarshal([]byte(v), workload); err != nil {
			log.Errorf(ctx, "[doGetWorkloads] failed to unmarshal %v, err: %v", k, err)
			return nil, err
		}
		workloads = append(workloads, workload)
	}

	return r.bindWorkloadsAdditions(ctx, workloads)
}

func (r *Rediaron) bindWorkloadsAdditions(ctx context.Context, workloads []*types.Workload) ([]*types.Workload, error) {
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
	ns, err := r.GetNodes(ctx, nodenames)
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
		v, err := r.GetOne(ctx, statusKeys[workload.ID])
		if err != nil {
			continue
		}
		status := &types.StatusMeta{}
		if err := json.Unmarshal([]byte(v), &status); err != nil {
			log.Warnf(ctx, "[bindWorkloadsAdditions] unmarshal %s status data failed %v", workload.ID, err)
			log.Errorf(ctx, "[bindWorkloadsAdditions] status raw: %s", v)
			continue
		}
		workloads[index].StatusMeta = status
	}
	return workloads, nil
}

func (r *Rediaron) doOpsWorkload(ctx context.Context, workload *types.Workload, create bool) error {
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
		err = r.BatchCreate(ctx, data)
	} else {
		err = r.BatchUpdate(ctx, data)
	}
	return err
}
