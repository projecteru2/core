package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// ReplaceContainer replace containers with same resource
func (c *Calcium) ReplaceContainer(ctx context.Context, opts *types.ReplaceOptions) (chan *types.ReplaceContainerMessage, error) {
	var oldContainers []*types.Container
	var err error
	if len(opts.IDs) == 0 {
		oldContainers, err = c.ListContainers(ctx, &types.ListContainersOptions{
			Appname: opts.Name, Entrypoint: opts.Entrypoint.Name, Nodename: opts.Nodename,
		})
	} else {
		oldContainers, err = c.GetContainers(ctx, opts.IDs)
	}
	if err != nil {
		return nil, err
	}

	ch := make(chan *types.ReplaceContainerMessage)

	go func() {
		defer close(ch)
		// 并发控制
		step := opts.Count
		wg := sync.WaitGroup{}
		ib := newImageBucket()
		defer wg.Wait()
		for index, oldContainer := range oldContainers {
			if opts.Podname != "" && oldContainer.Podname != opts.Podname {
				log.Debugf("[ReplaceContainer] Skip not in pod container %s", oldContainer.ID)
				continue
			}
			log.Debugf("[ReplaceContainer] Replace old container %s", oldContainer.ID)
			wg.Add(1)
			go func(replaceOpts types.ReplaceOptions, oldContainer *types.Container, index int) {
				defer wg.Done()
				// 使用复制之后的配置
				// 停老的，起新的
				replaceOpts.Memory = oldContainer.Memory
				replaceOpts.CPUQuota = oldContainer.Quota
				replaceOpts.SoftLimit = oldContainer.SoftLimit
				// 覆盖 podname 如果做全量更新的话
				replaceOpts.Podname = oldContainer.Podname

				createMessage, removeMessage, err := c.doReplaceContainer(
					ctx, oldContainer, &replaceOpts, ib, index,
				)
				ch <- &types.ReplaceContainerMessage{
					Create: createMessage,
					Remove: removeMessage,
					Error:  err,
				}
				if err != nil {
					log.Errorf("[ReplaceContainer] Replace and remove failed %v, old container restarted", err)
					return
				}
				log.Infof("[ReplaceContainer] Replace and remove success %s", oldContainer.ID)
				// 传 opts 的值，产生一次复制
			}(*opts, oldContainer, index)
			if (index+1)%step == 0 {
				wg.Wait()
			}
		}

		// 把收集的image清理掉
		//TODO 如果 remove 是异步的，这里就不能用 ctx 了，gRPC 一断这里就会死
		go c.cleanCachedImage(ctx, ib)
	}()

	return ch, nil
}

func (c *Calcium) doReplaceContainer(
	ctx context.Context,
	container *types.Container,
	opts *types.ReplaceOptions,
	ib *imageBucket,
	index int,
) (*types.CreateContainerMessage, *types.RemoveContainerMessage, error) {
	removeMessage := &types.RemoveContainerMessage{
		ContainerID: container.ID,
		Success:     false,
		Message:     "",
	}

	// 锁住，防止删除
	lock, err := c.Lock(ctx, fmt.Sprintf(cluster.ContainerLock, container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return nil, removeMessage, err
	}
	defer lock.Unlock(ctx)

	// 确保是有这个容器的
	containerJSON, err := container.Inspect(ctx)
	if err != nil {
		return nil, removeMessage, err
	}

	if !utils.FilterContainer(containerJSON.Config.Labels, opts.FilterLabels) {
		return nil, removeMessage, types.ErrNotFitLabels
	}

	// 拉镜像
	auth, err := makeEncodedAuthConfigFromRemote(c.config.Docker.AuthConfigs, opts.Image)
	if err != nil {
		return nil, removeMessage, err
	}

	if err = pullImage(ctx, container.Node, opts.Image, auth); err != nil {
		return nil, removeMessage, err
	}

	// 停止容器
	removeMessage.Message, err = c.doStopContainer(ctx, container, containerJSON, ib, opts.Force)
	if err != nil {
		return nil, removeMessage, err
	}

	// 获得文件 io
	for src, dst := range opts.Copy {
		stream, _, err := container.Engine.CopyFromContainer(ctx, container.ID, src)
		if err != nil {
			return nil, removeMessage, err
		}
		fname, err := utils.TempFile(stream)
		if err != nil {
			return nil, removeMessage, err
		}
		opts.DeployOptions.Data[dst] = fname
	}

	// 不涉及资源消耗，创建容器失败会被回收容器而不回收资源
	// 创建成功容器会干掉之前的老容器也不会动资源，实际上实现了动态捆绑
	createMessage := c.createAndStartContainer(ctx, index, container.Node, &opts.DeployOptions, container.CPU)
	if createMessage.Error != nil {
		// 重启老容器
		message, err := c.doStartContainer(ctx, container, containerJSON)
		removeMessage.Message += message
		if err != nil {
			log.Errorf("[replaceAndRemove] Old container %s restart failed %v", container.ID, err)
			removeMessage.Message += err.Error()
		}
		return nil, removeMessage, createMessage.Error
	}

	// 干掉老的
	if err = c.doRemoveContainer(ctx, container); err != nil {
		log.Errorf("[replaceAndRemove] Old container %s remove failed %v", container.ID, err)
		return createMessage, removeMessage, err
	}

	removeMessage.Success = true
	return createMessage, removeMessage, nil
}
