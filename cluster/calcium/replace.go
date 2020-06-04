package calcium

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"sync"

	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// ReplaceContainer replace containers with same resource
func (c *Calcium) ReplaceContainer(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceContainerMessage, error) {
	if opts.Count == 0 {
		opts.Count = 1
	}
	if len(opts.IDs) == 0 {
		oldContainers, err := c.ListContainers(ctx, &types.ListContainersOptions{
			Appname: opts.Name, Entrypoint: opts.Entrypoint.Name, Nodename: opts.Nodename,
		})
		if err != nil {
			return nil, err
		}
		for _, container := range oldContainers {
			opts.IDs = append(opts.IDs, container.ID)
		}
	}
	ch := make(chan *types.ReplaceContainerMessage)
	go func() {
		defer close(ch)
		// 并发控制
		wg := sync.WaitGroup{}
		defer wg.Wait()
		for index, ID := range opts.IDs {
			wg.Add(1)
			go func(replaceOpts types.ReplaceOptions, index int, ID string) {
				defer wg.Done()
				var createMessage *types.CreateContainerMessage
				removeMessage := &types.RemoveContainerMessage{ContainerID: ID}
				var err error
				if err = c.withContainerLocked(ctx, ID, func(container *types.Container) error {
					if opts.Podname != "" && container.Podname != opts.Podname {
						log.Warnf("[ReplaceContainer] Skip not in pod container %s", container.ID)
						return types.NewDetailedErr(types.ErrIgnoreContainer,
							fmt.Sprintf("container %s not in pod %s", container.ID, opts.Podname),
						)
					}
					// 使用复制之后的配置
					// 停老的，起新的
					replaceOpts.Memory = container.Memory
					replaceOpts.Storage = container.Storage
					replaceOpts.CPUQuota = container.Quota
					replaceOpts.SoftLimit = container.SoftLimit
					// 覆盖 podname 如果做全量更新的话
					replaceOpts.Podname = container.Podname
					// 覆盖 Volumes
					replaceOpts.Volumes = container.Volumes
					// 继承网络配置
					if replaceOpts.NetworkInherit {
						info, err := container.Inspect(ctx)
						if err != nil {
							return err
						} else if !info.Running {
							return types.NewDetailedErr(types.ErrNotSupport,
								fmt.Sprintf("container %s is not running, can not inherit", container.ID),
							)
						}
						replaceOpts.NetworkMode = ""
						replaceOpts.Networks = info.Networks
						log.Infof("[ReplaceContainer] Inherit old container network configuration mode %v", replaceOpts.Networks)
					}
					createMessage, removeMessage, err = c.doReplaceContainer(ctx, container, &replaceOpts, index)
					return err
				}); err != nil {
					if errors.Is(err, types.ErrIgnoreContainer) {
						return
					}
					log.Errorf("[ReplaceContainer] Replace and remove failed %v, old container restarted", err)
				} else {
					log.Infof("[ReplaceContainer] Replace and remove success %s", ID)
					log.Infof("[ReplaceContainer] New container %s", createMessage.ContainerID)
				}
				ch <- &types.ReplaceContainerMessage{Create: createMessage, Remove: removeMessage, Error: err}
			}(*opts, index, ID) // 传 opts 的值，产生一次复制
			if (index+1)%opts.Count == 0 {
				wg.Wait()
			}
		}
	}()

	return ch, nil
}

func (c *Calcium) doReplaceContainer(
	ctx context.Context,
	container *types.Container,
	opts *types.ReplaceOptions,
	index int,
) (*types.CreateContainerMessage, *types.RemoveContainerMessage, error) {
	removeMessage := &types.RemoveContainerMessage{
		ContainerID: container.ID,
		Success:     false,
		Hook:        []*bytes.Buffer{},
	}
	// label filter
	if !utils.FilterContainer(container.Labels, opts.FilterLabels) {
		return nil, removeMessage, types.ErrNotFitLabels
	}
	// get node
	node, err := c.GetNode(ctx, container.Nodename)
	if err != nil {
		return nil, removeMessage, err
	}
	// pull image
	if err = pullImage(ctx, node, opts.Image); err != nil {
		return nil, removeMessage, err
	}
	// 获得文件 io
	for src, dst := range opts.Copy {
		stream, _, err := container.Engine.VirtualizationCopyFrom(ctx, container.ID, src)
		if err != nil {
			return nil, removeMessage, err
		}
		bs, err := ioutil.ReadAll(stream)
		if err != nil {
			return nil, removeMessage, err
		}
		opts.DeployOptions.Data[dst] = bytes.NewReader(bs)
	}

	createMessage := &types.CreateContainerMessage{}
	return createMessage, removeMessage, c.Transaction(
		ctx,
		// if
		func(ctx context.Context) (err error) {
			removeMessage.Hook, err = c.doStopContainer(ctx, container, opts.IgnoreHook)
			return
		},
		// then
		func(ctx context.Context) error {
			return c.Transaction(
				ctx,
				// if
				func(ctx context.Context) error {
					createMessage = c.doCreateAndStartContainer(ctx, index, node, &opts.DeployOptions, container.CPU, container.VolumePlan)
					return createMessage.Error
				},
				// then
				func(ctx context.Context) (err error) {
					if err = c.doRemoveContainer(ctx, container, true); err != nil {
						log.Errorf("[replaceAndRemove] the new started but the old failed to stop")
						return
					}
					removeMessage.Success = true
					return
				},
				// else
				nil,
			)
		},
		// else
		func(ctx context.Context) (err error) {
			messages, err := c.doStartContainer(ctx, container, opts.IgnoreHook)
			log.Errorf("[replaceAndRemove] Old container %s restart failed %v", container.ID, err)
			removeMessage.Hook = append(removeMessage.Hook, messages...)
			if err != nil {
				removeMessage.Hook = append(removeMessage.Hook, bytes.NewBufferString(err.Error()))
			}
			return
		},
	)
}
