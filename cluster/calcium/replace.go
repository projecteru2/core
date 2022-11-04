package calcium

import (
	"bytes"
	"context"
	"sync"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/cockroachdb/errors"
)

// ReplaceWorkload replace workloads with same resource
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
		// 并发控制
		wg := sync.WaitGroup{}
		wg.Add(len(opts.IDs))
		defer wg.Wait()
		for index, ID := range opts.IDs {
			_ = c.pool.Invoke(func(replaceOpts types.ReplaceOptions, index int, ID string) func() {
				return func() {
					_ = c.pool.Invoke(func() {
						defer wg.Done()
						var createMessage *types.CreateWorkloadMessage
						removeMessage := &types.RemoveWorkloadMessage{WorkloadID: ID}
						var err error
						if err = c.withWorkloadLocked(ctx, ID, func(ctx context.Context, workload *types.Workload) error {
							if opts.Podname != "" && workload.Podname != opts.Podname {
								logger.Warnf(ctx, "Skip not in pod workload %s", workload.ID)
								return errors.Wrapf(types.ErrWorkloadIgnored, "workload %s not in pod %s", workload.ID, opts.Podname)
							}
							// 使用复制之后的配置
							// 停老的，起新的
							// replaceOpts.ResourceOpts = workload.ResourceUsage
							// 覆盖 podname 如果做全量更新的话
							replaceOpts.Podname = workload.Podname
							// 覆盖 Volumes
							// 继承网络配置
							if replaceOpts.NetworkInherit {
								info, err := workload.Inspect(ctx)
								if err != nil {
									return err
								} else if !info.Running {
									return errors.Wrapf(types.ErrInvaildWorkloadOps, "workload %s is not running, can not inherit", workload.ID)
								}
								replaceOpts.Networks = info.Networks
								logger.Infof(ctx, "Inherit old workload network configuration mode %+v", replaceOpts.Networks)
							}
							createMessage, removeMessage, err = c.doReplaceWorkload(ctx, workload, &replaceOpts, index)
							return err
						}); err != nil {
							if errors.Is(err, types.ErrWorkloadIgnored) {
								logger.Warnf(ctx, "ignore workload: %+v", err)
								return
							}
							logger.Error(ctx, err, "Replace and remove failed, old workload restarted")
						} else {
							logger.Infof(ctx, "Replace and remove success %s", ID)
							logger.Infof(ctx, "New workload %s", createMessage.WorkloadID)
						}
						ch <- &types.ReplaceWorkloadMessage{Create: createMessage, Remove: removeMessage, Error: err}
					})
				}
			}(*opts, index, ID))
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
	// label filter
	if !utils.LabelsFilter(workload.Labels, opts.FilterLabels) {
		return nil, removeMessage, types.ErrWorkloadIgnored
	}
	// prepare node
	node, err := c.doGetAndPrepareNode(ctx, workload.Nodename, opts.Image)
	if err != nil {
		return nil, removeMessage, err
	}
	// 获得文件 io
	for src, dst := range opts.Copy {
		content, uid, gid, mode, err := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, src)
		if err != nil {
			return nil, removeMessage, err
		}
		opts.DeployOptions.Files = append(opts.DeployOptions.Files, types.LinuxFile{
			Filename: dst,
			Content:  content,
			UID:      uid,
			GID:      gid,
			Mode:     mode,
		})
	}

	// copy resource args
	createMessage := &types.CreateWorkloadMessage{
		ResourceArgs: map[string]types.WorkloadResourceArgs{},
		EngineArgs:   workload.EngineArgs,
	}
	for plugin, rawParams := range workload.ResourceArgs {
		createMessage.ResourceArgs[plugin] = rawParams
	}

	if err = utils.Txn(
		ctx,
		// if
		func(ctx context.Context) (err error) {
			removeMessage.Hook, err = c.doStopWorkload(ctx, workload, opts.IgnoreHook)
			return err
		},
		// then
		func(ctx context.Context) error {
			return utils.Txn(
				ctx,
				// if
				func(ctx context.Context) error {
					vco := c.doMakeReplaceWorkloadOptions(ctx, index, createMessage, &opts.DeployOptions, node, workload.ID)
					return c.doDeployOneWorkload(ctx, node, &opts.DeployOptions, createMessage, vco, false)
				},
				// then
				func(ctx context.Context) (err error) {
					if err = c.doRemoveWorkload(ctx, workload, true); err != nil {
						logger.Error(ctx, err, "the new started but the old failed to stop")
						return err
					}
					removeMessage.Success = true
					return
				},
				nil,
				c.config.GlobalTimeout,
			)
		},
		// rollback
		func(ctx context.Context, _ bool) (err error) {
			messages, err := c.doStartWorkload(ctx, workload, opts.IgnoreHook)
			if err != nil {
				logger.Error(ctx, err, "Old workload %s restart failed", workload.ID)
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

	_ = c.pool.Invoke(func() { c.doRemapResourceAndLog(ctx, logger, node) })

	return createMessage, removeMessage, err
}

func (c *Calcium) doMakeReplaceWorkloadOptions(ctx context.Context, no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node, ancestorWorkloadID string) *enginetypes.VirtualizationCreateOptions {
	vco := c.doMakeWorkloadOptions(ctx, no, msg, opts, node)
	vco.AncestorWorkloadID = ancestorWorkloadID
	return vco
}
