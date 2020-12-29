package calcium

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"sync"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// ReplaceWorkload replace workloads with same resource
func (c *Calcium) ReplaceWorkload(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceWorkloadMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	opts.Normalize()
	if len(opts.IDs) == 0 {
		if len(opts.Nodenames) == 0 {
			opts.Nodenames = []string{""}
		}
		for _, nodename := range opts.Nodenames {
			workloads, err := c.ListWorkloads(ctx, &types.ListWorkloadsOptions{
				Appname: opts.Name, Entrypoint: opts.Entrypoint.Name, Nodename: nodename,
			})
			if err != nil {
				return nil, err
			}
			for _, workload := range workloads {
				opts.IDs = append(opts.IDs, workload.ID)
			}
		}
	}
	ch := make(chan *types.ReplaceWorkloadMessage)
	go func() {
		defer close(ch)
		// 并发控制
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for index, id := range opts.IDs {
			wg.Add(1)
			go func(replaceOpts types.ReplaceOptions, index int, id string) {
				defer wg.Done()
				var createMessage *types.CreateWorkloadMessage
				removeMessage := &types.RemoveWorkloadMessage{WorkloadID: id}
				var err error
				if err = c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
					if opts.Podname != "" && workload.Podname != opts.Podname {
						log.Warnf("[ReplaceWorkload] Skip not in pod workload %s", workload.ID)
						return types.NewDetailedErr(types.ErrIgnoreWorkload,
							fmt.Sprintf("workload %s not in pod %s", workload.ID, opts.Podname),
						)
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
							return types.NewDetailedErr(types.ErrNotSupport,
								fmt.Sprintf("workload %s is not running, can not inherit", workload.ID),
							)
						}
						replaceOpts.Networks = info.Networks
						log.Infof("[ReplaceWorkload] Inherit old workload network configuration mode %v", replaceOpts.Networks)
					}
					createMessage, removeMessage, err = c.doReplaceWorkload(ctx, workload, &replaceOpts, index)
					return err
				}); err != nil {
					if errors.Is(err, types.ErrIgnoreWorkload) {
						return
					}
					log.Errorf("[ReplaceWorkload] Replace and remove failed %v, old workload restarted", err)
				} else {
					log.Infof("[ReplaceWorkload] Replace and remove success %s", id)
					log.Infof("[ReplaceWorkload] New workload %s", createMessage.WorkloadID)
				}
				ch <- &types.ReplaceWorkloadMessage{Create: createMessage, Remove: removeMessage, Error: err}
			}(*opts, index, id) // 传 opts 的值，产生一次复制
			if (index+1)%opts.Count == 0 {
				wg.Wait()
			}
		}
	}()
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
		return nil, removeMessage, types.ErrNotFitLabels
	}
	// prepare node
	node, err := c.doGetAndPrepareNode(ctx, workload.Nodename, opts.Image)
	if err != nil {
		return nil, removeMessage, err
	}
	// 获得文件 io
	for src, dst := range opts.Copy {
		stream, _, err := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, src)
		if err != nil {
			return nil, removeMessage, err
		}
		if opts.DeployOptions.Data[dst], err = types.NewReaderManager(stream); err != nil {
			return nil, removeMessage, err
		}
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
	return createMessage, removeMessage, utils.Txn(
		ctx,
		// if
		func(ctx context.Context) (err error) {
			removeMessage.Hook, err = c.doStopWorkload(ctx, workload, opts.IgnoreHook)
			return
		},
		// then
		func(ctx context.Context) error {
			return utils.Txn(
				ctx,
				// if
				func(ctx context.Context) error {
					vco := c.doMakeReplaceWorkloadOptions(index, createMessage, &opts.DeployOptions, node, workload.ID)
					return c.doDeployOneWorkload(ctx, node, &opts.DeployOptions, createMessage, vco, -1)
				},
				// then
				func(ctx context.Context) (err error) {
					if err = c.doRemoveWorkload(ctx, workload, true); err != nil {
						log.Errorf("[doReplaceWorkload] the new started but the old failed to stop")
						return
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
				log.Errorf("[replaceAndRemove] Old workload %s restart failed %v", workload.ID, err)
				removeMessage.Hook = append(removeMessage.Hook, bytes.NewBufferString(err.Error()))
			} else {
				removeMessage.Hook = append(removeMessage.Hook, messages...)
			}
			return
		},
		c.config.GlobalTimeout,
	)
}

func (c *Calcium) doMakeReplaceWorkloadOptions(no int, msg *types.CreateWorkloadMessage, opts *types.DeployOptions, node *types.Node, ancestorWorkloadID string) *enginetypes.VirtualizationCreateOptions {
	vco := c.doMakeWorkloadOptions(no, msg, opts, node)
	vco.AncestorWorkloadID = ancestorWorkloadID
	return vco
}
