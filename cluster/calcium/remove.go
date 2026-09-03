package calcium

import (
	"bytes"
	"context"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) RemoveWorkload(ctx context.Context, IDs []string, force bool) (chan *types.RemoveWorkloadMessage, error) {
	logger := log.WithFunc("calcium.RemoveWorkload").WithField("IDs", IDs).WithField("force", force)
	ch := make(chan *types.RemoveWorkloadMessage)
	err := c.releaseWorkloads(ctx, logger, IDs, func(ctx context.Context, _ *types.Node, workload *types.Workload) error {
		if err := c.doRemoveOneWorkload(ctx, workload, force); err != nil {
			return err
		}
		logger.Infof(ctx, "workload %s removed", workload.ID)
		return nil
	}, func(workloadID string, err error) error {
		ret := &types.RemoveWorkloadMessage{WorkloadID: workloadID, Success: err == nil, Hook: []*bytes.Buffer{}}
		if err != nil {
			logger.WithField("id", workloadID).Error(ctx, err, "failed to remove workload")
			ret.Hook = append(ret.Hook, bytes.NewBufferString(err.Error()))
		}
		return send(ctx, ch, ret)
	}, func() { close(ch) })
	if err != nil {
		logger.Error(ctx, err, "failed to group workloads by node")
		return nil, err
	}
	return ch, nil
}

func (c *Calcium) doRemoveOneWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	logger := log.WithFunc("calcium.doRemoveOneWorkload").WithField("id", workload.ID)
	workloadCommit, err := c.journal(ctx, logger, eventWorkloadCreated, &types.Workload{ID: workload.ID, Name: workload.Name, Nodename: workload.Nodename})
	if err != nil {
		return err
	}
	defer workloadCommit()
	return c.doRemoveWorkload(ctx, workload, force)
}

func (c *Calcium) doRemoveWorkload(ctx context.Context, workload *types.Workload, force bool) error {
	_, err := utils.Txn(
		ctx,
		func(ctx context.Context) error {
			return c.store.RemoveWorkload(ctx, workload)
		},
		func(ctx context.Context) error {
			return workload.Remove(ctx, force)
		},
		func(ctx context.Context, failedByCond bool) error {
			if failedByCond {
				return nil
			}
			return c.store.AddWorkload(ctx, workload, nil)
		},
		c.config.GlobalTimeout,
	)
	return err
}

func (c *Calcium) doRemoveWorkloadSync(ctx context.Context, IDs []string) error {
	ch, err := c.RemoveWorkload(ctx, IDs, true)
	if err != nil {
		return err
	}

	logger := log.WithFunc("calcium.doRemoveWorkloadSync")
	errs := []error{}
	for m := range ch {
		if !m.Success {
			errs = append(errs, errors.Newf("failed to remove workload %s: %s", m.WorkloadID, utils.MergeHookOutputs(m.Hook)))
			continue
		}
		logger.Debugf(ctx, "removed %s", m.WorkloadID)
	}
	return errors.Join(errs...)
}

func (c *Calcium) groupWorkloadsByNode(ctx context.Context, IDs []string) (map[string][]string, error) {
	workloads, err := c.store.GetWorkloads(ctx, IDs)
	if err != nil {
		return nil, err
	}
	nodeWorkloadGroup := map[string][]string{}
	for _, workload := range workloads {
		nodeWorkloadGroup[workload.Nodename] = append(nodeWorkloadGroup[workload.Nodename], workload.ID)
	}
	return nodeWorkloadGroup, nil
}
