package calcium

import (
	"bytes"
	"context"
	"slices"
	"sync"

	"github.com/cockroachdb/errors"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *Calcium) ReplaceWorkload(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceWorkloadMessage, error) {
	logger := log.WithFunc("calcium.ReplaceWorkload").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	opts.Normalize()
	if len(opts.IDs) == 0 {
		if len(opts.NodeFilter.Includes) == 0 {
			opts.NodeFilter.Includes = []string{""}
		}
		for _, nodename := range opts.NodeFilter.Includes {
			workloads, err := c.ListWorkloads(ctx, &types.ListWorkloadsOptions{
				Appname: opts.Name, Entrypoint: opts.Entrypoint.Name, Nodename: nodename,
			})
			if err != nil {
				logger.Error(ctx, err)
				return nil, err
			}
			for _, workload := range workloads {
				opts.IDs = append(opts.IDs, workload.ID)
			}
		}
	}
	ch := make(chan *types.ReplaceWorkloadMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		wg.Add(len(opts.IDs))
		defer wg.Wait()
		for index, ID := range opts.IDs {
			replaceOpts := *opts
			_ = c.pool.Invoke(func() {
				defer wg.Done()
				var createMessage *types.CreateWorkloadMessage
				removeMessage := &types.RemoveWorkloadMessage{WorkloadID: ID}
				var err error
				if err = c.withWorkloadLocked(ctx, ID, false, func(ctx context.Context, workload *types.Workload) error {
					if opts.Podname != "" && workload.Podname != opts.Podname {
						logger.Warnf(ctx, "skip workload %s not in pod", workload.ID)
						return errors.Wrapf(types.ErrWorkloadIgnored, "workload %s not in pod %s", workload.ID, opts.Podname)
					}
					replaceOpts.Podname = workload.Podname
					if replaceOpts.NetworkInherit {
						info, inspectErr := workload.Inspect(ctx)
						if inspectErr != nil {
							return inspectErr
						} else if !info.Running {
							return errors.Wrapf(types.ErrInvaildWorkloadOps, "workload %s is not running, can not inherit", workload.ID)
						}
						replaceOpts.Networks = info.Networks
						logger.Infof(ctx, "inherit old workload network configuration %+v", replaceOpts.Networks)
					}
					createMessage, removeMessage, err = c.doReplaceWorkload(ctx, workload, &replaceOpts, index)
					return err
				}); err != nil {
					if errors.Is(err, types.ErrWorkloadIgnored) {
						logger.Warnf(ctx, "ignore workload: %+v", err)
						return
					}
					logger.Error(ctx, err, "replace and remove failed, old workload restarted")
				} else {
					logger.Infof(ctx, "replaced workload %s with %s", ID, createMessage.WorkloadID)
				}
				ch <- &types.ReplaceWorkloadMessage{Create: createMessage, Remove: removeMessage, Error: err}
			})
		}
	})
	return ch, nil
}

func (c *Calcium) doReplaceWorkload(
	ctx context.Context,
	workload *types.Workload,
	opts *types.ReplaceOptions,
	index int,
) (*types.CreateWorkloadMessage, *types.RemoveWorkloadMessage, error) {
	removeMessage := &types.RemoveWorkloadMessage{
		WorkloadID: workload.ID,
		Success:    false,
		Hook:       []*bytes.Buffer{},
	}
	logger := log.WithFunc("calcium.doReplaceWorkload")
	if !utils.LabelsFilter(workload.Labels, opts.FilterLabels) {
		return nil, removeMessage, types.ErrWorkloadIgnored
	}
	node, err := c.doGetAndPrepareNode(ctx, workload.Nodename, opts.Image, opts.IgnorePull)
	if err != nil {
		return nil, removeMessage, err
	}
	files := slices.Clone(opts.Files)
	for src, dst := range opts.Copy {
		content, uid, gid, mode, copyErr := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, src)
		if copyErr != nil {
			return nil, removeMessage, copyErr
		}
		files = append(files, types.LinuxFile{
			Filename: dst,
			Content:  content,
			UID:      uid,
			GID:      gid,
			Mode:     mode,
		})
	}
	opts.Files = files

	createMessage := &types.CreateWorkloadMessage{
		Resources:    workload.Resources,
		EngineParams: workload.EngineParams,
	}

	if _, err = utils.Txn(
		ctx,
		func(ctx context.Context) (err error) {
			removeMessage.Hook, err = c.doStopWorkload(ctx, workload, opts.IgnoreHook)
			return err
		},
		func(ctx context.Context) error {
			_, txnErr := utils.Txn(
				ctx,
				func(ctx context.Context) error {
					vco := c.doMakeWorkloadOptions(ctx, index, createMessage, &opts.DeployOptions, node)
					vco.AncestorWorkloadID = workload.ID
					return c.doDeployOneWorkload(ctx, node, &opts.DeployOptions, createMessage, vco, false)
				},
				func(ctx context.Context) (err error) {
					commit, err := c.journal(ctx, logger, eventWorkloadReplaced, &workloadReplacement{OldID: workload.ID, NewID: createMessage.WorkloadID})
					if err != nil {
						return err
					}
					if err = c.doRemoveWorkload(ctx, workload, true); err != nil {
						logger.Error(ctx, err, "the new started but the old failed to stop")
						return err
					}
					commit()
					removeMessage.Success = true
					return nil
				},
				nil,
				c.config.GlobalTimeout,
			)
			return txnErr
		},
		func(ctx context.Context, _ bool) (err error) {
			messages, err := c.doStartWorkload(ctx, workload, opts.IgnoreHook)
			if err != nil {
				logger.Errorf(ctx, err, "old workload %s restart failed", workload.ID)
				removeMessage.Hook = append(removeMessage.Hook, bytes.NewBufferString(err.Error()))
			} else {
				removeMessage.Hook = append(removeMessage.Hook, messages...)
			}
			return err
		},
		c.config.GlobalTimeout,
	); err != nil {
		return createMessage, removeMessage, err
	}

	_ = c.pool.Invoke(func() { c.RemapResourceAndLog(ctx, logger, node) })

	return createMessage, removeMessage, err
}
