package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/store/common"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (r *Rediaron) AddWorkload(ctx context.Context, workload *types.Workload, processing *types.Processing) error {
	return r.doOpsWorkload(ctx, workload, processing, true)
}

func (r *Rediaron) UpdateWorkload(ctx context.Context, workload *types.Workload) error {
	return r.doOpsWorkload(ctx, workload, nil, false)
}

func (r *Rediaron) RemoveWorkload(ctx context.Context, workload *types.Workload) error {
	keys, err := common.WorkloadKeys(workload)
	if err != nil {
		return err
	}
	return r.BatchDelete(ctx, keys)
}

func (r *Rediaron) GetWorkload(ctx context.Context, ID string) (*types.Workload, error) {
	workloads, err := r.GetWorkloads(ctx, []string{ID})
	if err != nil {
		return nil, err
	}
	return workloads[0], nil
}

func (r *Rediaron) GetWorkloads(ctx context.Context, IDs []string) (workloads []*types.Workload, err error) {
	keys := []string{}
	for _, ID := range IDs {
		keys = append(keys, fmt.Sprintf(common.WorkloadInfoKey, ID))
	}

	return r.doGetWorkloads(ctx, keys)
}

func (r *Rediaron) GetWorkloadStatus(ctx context.Context, ID string) (*types.StatusMeta, error) {
	workload, err := r.GetWorkload(ctx, ID)
	if err != nil {
		return nil, err
	}
	return workload.StatusMeta, nil
}

func (r *Rediaron) SetWorkloadStatus(ctx context.Context, status *types.StatusMeta, ttl int64) error {
	return common.SetWorkloadStatus(ctx, r, status, ttl)
}

func (r *Rediaron) ListWorkloads(ctx context.Context, appname, entrypoint, nodename string, limit int64, labels map[string]string) ([]*types.Workload, error) {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// trailing slash keeps the prefix from matching a longer nodename
	key := filepath.Join(common.WorkloadDeployPrefix, appname, entrypoint, nodename) + "/*"
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
		if utils.LabelsFilter(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return r.bindWorkloadsAdditions(ctx, workloads)
}

func (r *Rediaron) ListNodeWorkloads(ctx context.Context, nodename string, labels map[string]string) ([]*types.Workload, error) {
	key := fmt.Sprintf(common.NodeWorkloadsKey, nodename, "*")
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
		if utils.LabelsFilter(workload.Labels, labels) {
			workloads = append(workloads, workload)
		}
	}

	return r.bindWorkloadsAdditions(ctx, workloads)
}

func (r *Rediaron) WorkloadStatusStream(ctx context.Context, appname, entrypoint, nodename string, labels map[string]string) chan *types.WorkloadStatus {
	if appname == "" {
		entrypoint = ""
	}
	if entrypoint == "" {
		nodename = ""
	}
	// trailing slash keeps the prefix from matching a longer nodename
	statusKey := filepath.Join(common.WorkloadStatusPrefix, appname, entrypoint, nodename) + "/*"
	ch := make(chan *types.WorkloadStatus)
	logger := log.WithFunc("store.redis.WorkloadStatusStream")
	if err := r.pool.Invoke(func() {
		defer func() {
			logger.Info(ctx, "close WorkloadStatus channel")
			close(ch)
		}()

		logger.Infof(ctx, "watch on %s", statusKey)
		for message := range r.KNotify(ctx, statusKey) {
			_, _, _, ID := common.ParseStatusKey(message.Key)
			msg := &types.WorkloadStatus{
				ID:     ID,
				Delete: message.Action == actionDel || message.Action == actionExpired,
			}
			workload, err := r.GetWorkload(ctx, ID)
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
	}); err != nil {
		logger.Error(ctx, err, "invoke watcher")
		close(ch)
	}
	return ch
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
			log.WithFunc("store.redis.doGetWorkloads").Errorf(ctx, err, "failed to unmarshal %+v", k)
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
	logger := log.WithFunc("store.redis.bindWorkloadsAdditions")
	for _, workload := range workloads {
		appname, entrypoint, _, err := utils.ParseWorkloadName(workload.Name)
		if err != nil {
			return nil, err
		}
		statusKeys[workload.ID] = filepath.Join(common.WorkloadStatusPrefix, appname, entrypoint, workload.Nodename, workload.ID)
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
			return nil, types.ErrInvaildWorkloadMeta
		}
		workloads[index].Engine = nodes[workload.Nodename].Engine
		v, err := r.GetOne(ctx, statusKeys[workload.ID])
		if err != nil {
			continue
		}
		status := &types.StatusMeta{}
		if err := json.Unmarshal([]byte(v), &status); err != nil {
			logger.Errorf(ctx, err, "unmarshal status of %s, raw: %s", workload.ID, v)
			continue
		}
		workloads[index].StatusMeta = status
	}
	return workloads, nil
}

func (r *Rediaron) doOpsWorkload(ctx context.Context, workload *types.Workload, processing *types.Processing, create bool) error {
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
		fmt.Sprintf(common.WorkloadInfoKey, workload.ID):                                                workloadData,
		fmt.Sprintf(common.NodeWorkloadsKey, workload.Nodename, workload.ID):                            workloadData,
		filepath.Join(common.WorkloadDeployPrefix, appname, entrypoint, workload.Nodename, workload.ID): workloadData,
	}

	if create {
		if processing != nil {
			err = r.BatchCreateAndDecr(ctx, data, common.ProcessingKey(processing))
		} else {
			err = r.BatchCreate(ctx, data)
		}
	} else {
		err = r.BatchUpdate(ctx, data)
	}
	return err
}
