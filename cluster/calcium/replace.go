package calcium

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// ReplaceWorkload replace workloads with same resource
func (c *Calcium) ReplaceWorkload(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceWorkloadMessage, error) {
	logger := log.WithField("Calcium", "ReplaceWorkload").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(ctx, err)
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
				return nil, logger.Err(ctx, err)
			}
			for _, workload := range workloads {
				opts.IDs = append(opts.IDs, workload.ID)
			}
		}
	}
	ch := make(chan *types.ReplaceWorkloadMessage)
	utils.SentryGo(func() {
		defer close(ch)
		// 并发控制
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for index, id := range opts.IDs {
			wg.Add(1)
			utils.SentryGo(
				func(replaceOpts types.ReplaceOptions, index int, id string) func() {
					return func() {
						defer wg.Done()
						var createMessage *types.CreateWorkloadMessage
						removeMessage := &types.RemoveWorkloadMessage{WorkloadID: id}
						var err error
						if err = c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
							if opts.Podname != "" && workload.Podname != opts.Podname {
								log.Warnf(ctx, "[ReplaceWorkload] Skip not in pod workload %s", workload.ID)
								return errors.WithStack(types.NewDetailedErr(types.ErrIgnoreWorkload,
									fmt.Sprintf("workload %s not in pod %s", workload.ID, opts.Podname),
								))
							}
							// 使用复制之后的配置
							// 停老的，起新的
							replaceOpts.ResourceOpts = types.ResourceOptions{
								CPUQuotaRequest: workload.CPUQuotaRequest,
								CPUQuotaLimit:   workload.CPUQuotaLimit,
								CPUBind:         len(workload.CPU) > 0,
								MemoryRequest:   workload.MemoryRequest,
								MemoryLimit:     workload.MemoryLimit,
								StorageRequest:  workload.StorageRequest,
								StorageLimit:    workload.StorageLimit,
								VolumeRequest:   workload.VolumeRequest,
								VolumeLimit:     workload.VolumeLimit,
							}
							// 覆盖 podname 如果做全量更新的话
							replaceOpts.Podname = workload.Podname
							// 覆盖 Volumes
							// 继承网络配置
							if replaceOpts.NetworkInherit {
								info, err := workload.Inspect(ctx)
								if err != nil {
									return err
								} else if !info.Running {
									return errors.WithStack(types.NewDetailedErr(types.ErrNotSupport,
										fmt.Sprintf("workload %s is not running, can not inherit", workload.ID),
									))
								}
								replaceOpts.Networks = info.Networks
								log.Infof(ctx, "[ReplaceWorkload] Inherit old workload network configuration mode %v", replaceOpts.Networks)
							}
							createMessage, removeMessage, err = c.doReplaceWorkload(ctx, workload, &replaceOpts, index)
							return err
						}); err != nil {
							if errors.Is(err, types.ErrIgnoreWorkload) {
								log.Warnf(ctx, "[ReplaceWorkload] ignore workload: %v", err)
								return
							}
							logger.Errorf(ctx, "[ReplaceWorkload] Replace and remove failed %+v, old workload restarted", err)
						} else {
							log.Infof(ctx, "[ReplaceWorkload] Replace and remove success %s", id)
							log.Infof(ctx, "[ReplaceWorkload] New workload %s", createMessage.WorkloadID)
						}
						ch <- &types.ReplaceWorkloadMessage{Create: createMessage, Remove: removeMessage, Error: err}
					}
				}(*opts, index, id))
			if (index+1)%opts.Count == 0 {
				wg.Wait()
			}
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
	// label filter
	if !utils.FilterWorkload(workload.Labels, opts.FilterLabels) {
		return nil, removeMessage, errors.WithStack(types.ErrNotFitLabels)
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
			return nil, removeMessage, errors.WithStack(err)
		}
		opts.DeployOptions.Files = append(opts.DeployOptions.Files, types.LinuxFile{
			Filename: dst,
			Content:  content,
			UID:      uid,
			GID:      gid,
			Mode:     mode,
		})
	}

	createMessage := &types.CreateWorkloadMessage{
		ResourceMeta: types.ResourceMeta{
			MemoryRequest:     workload.MemoryRequest,
			MemoryLimit:       workload.MemoryLimit,
			StorageRequest:    workload.StorageRequest,
			StorageLimit:      workload.StorageLimit,
			CPUQuotaRequest:   workload.CPUQuotaRequest,
			CPUQuotaLimit:     workload.CPUQuotaLimit,
			CPU:               workload.CPU,
			VolumeRequest:     workload.VolumeRequest,
			VolumePlanRequest: workload.VolumePlanRequest,
			VolumeLimit:       workload.VolumeLimit,
			VolumePlanLimit:   workload.VolumePlanLimit,
		},
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
						log.Errorf(ctx, "[doReplaceWorkload] the new started but the old failed to stop")
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
				log.Errorf(ctx, "[replaceAndRemove] Old workload %s restart failed %v", workload.ID, err)
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

	go c.doRemapResourceAndLog(ctx, log.WithField("Calcium", "doReplaceWorkload"), node)

	return createMessage, removeMessage, err
}

func (c *Calcium) doMakeReplaceWorkloadOptions(ctx context.Context, no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node, ancestorWorkloadID string) *enginetypes.VirtualizationCreateOptions {
	vco := c.doMakeWorkloadOptions(ctx, no, msg, opts, node)
	vco.AncestorWorkloadID = ancestorWorkloadID
	return vco
}
