package calcium

import (
	"context"
	"fmt"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ReplaceContainer replace containers with same resource
func (c *Calcium) ReplaceContainer(ctx context.Context, opts *types.DeployOptions, force bool) (chan *types.ReplaceContainerMessage, error) {
	oldContainers, err := c.ListContainers(ctx, opts.Name, opts.Entrypoint.Name, opts.Nodename)
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
			go func(deployOpts types.DeployOptions, oldContainer *types.Container, index int) {
				defer wg.Done()
				// 使用复制之后的配置
				// 停老的，起新的
				deployOpts.Memory = oldContainer.Memory
				deployOpts.CPUQuota = oldContainer.Quota
				deployOpts.SoftLimit = oldContainer.SoftLimit

				createMessage, removeMessage, err := c.doReplaceContainer(ctx, oldContainer, &deployOpts, ib, index, force)
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
	opts *types.DeployOptions,
	ib *imageBucket,
	index int,
	force bool,
) (*types.CreateContainerMessage, *types.RemoveContainerMessage, error) {
	removeMessage := &types.RemoveContainerMessage{
		ContainerID: container.ID,
		Success:     false,
		Message:     "",
	}

	// 锁住，防止删除
	lock, err := c.Lock(ctx, fmt.Sprintf("rmcontainer_%s", container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return nil, removeMessage, err
	}
	defer lock.Unlock(ctx)

	// 确保是有这个容器的
	containerJSON, err := container.Inspect(ctx)
	if err != nil {
		return nil, removeMessage, err
	}

	// 覆盖 podname 如果做全量更新的话
	opts.Podname = container.Podname

	// 记录镜像
	if ib != nil {
		ib.Add(container.Podname, containerJSON.Config.Image)
	}

	removeMessage.Message, err = c.doStopContainer(ctx, container, containerJSON, ib, force)
	if err != nil {
		return nil, removeMessage, err
	}

	// 拉镜像
	auth, err := makeEncodedAuthConfigFromRemote(c.config.Docker.AuthConfigs, opts.Image)
	if err != nil {
		return nil, removeMessage, err
	}

	if err = pullImage(ctx, container.Node, opts.Image, auth); err != nil {
		return nil, removeMessage, err
	}

	// 不涉及资源消耗，创建容器失败会被回收容器而不回收资源
	// 创建成功容器会干掉之前的老容器也不会动资源，实际上实现了动态捆绑
	createMessage := c.createAndStartContainer(ctx, index, container.Node, opts, container.CPU)
	if createMessage.Error != nil {
		// 重启老容器, 并不关心是否启动成功
		// 注意要再次激发 hook
		if err = container.Engine.ContainerStart(ctx, container.ID, enginetypes.ContainerStartOptions{}); err != nil {
			log.Errorf("[replaceAndRemove] Old container %s restart failed %v", container.ID, err)
		}

		if container.Hook != nil && len(container.Hook.AfterStart) > 0 {
			output, err := c.doContainerAfterStartHook(
				ctx, container,
				containerJSON.Config.User,
				containerJSON.Config.Env,
				container.Privileged,
			)
			log.Infof("[replaceAndRemove] Do after start hook %s", output)
			removeMessage.Message += string(output)
			if err != nil {
				log.Errorf("[replaceAndRemove] Old container %s after hook failed %v", container.ID, err)
			}
		}
		return nil, removeMessage, createMessage.Error
	}

	//TODO healthcheck

	// 干掉老的
	if err = c.doRemoveContainer(ctx, container); err != nil {
		log.Errorf("[replaceAndRemove] Old container %s remove failed %v", container.ID, err)
		return createMessage, removeMessage, err
	}

	removeMessage.Success = true
	return createMessage, removeMessage, nil
}
