package calcium

import (
	"context"
	"fmt"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ReplaceContainer replace containers with same resource
func (c *Calcium) ReplaceContainer(ctx context.Context, opts *types.DeployOptions) (chan *types.ReplaceContainerMessage, error) {
	ch := make(chan *types.ReplaceContainerMessage)
	oldContainers, err := c.ListContainers(ctx, opts.Name, opts.Entrypoint.Name, opts.Nodename)
	if err != nil {
		return ch, err
	}

	go func() {
		defer close(ch)
		lock, err := c.Lock(ctx, opts.Podname, c.config.LockTimeout)
		if err != nil {
			log.Errorf("[ReplaceContainer] Lock pod failed %v", err)
		}
		defer lock.Unlock(ctx)

		for index, oldContainer := range oldContainers {
			log.Debug("[ReplaceContainer] Replace old container : %s", oldContainer.ID)
			// 停老的，起新的
			createMessage, err := c.replaceAndRemove(ctx, oldContainer, opts, index)
			ch <- &types.ReplaceContainerMessage{
				CreateContainerMessage: createMessage,
				OldContainerID:         oldContainer.ID,
				Error:                  err,
			}
			if err != nil {
				log.Errorf("[ReplaceContainer] Replace and remove failed %v, old container restarted", err)
				// 立即退出
				return
			}
		}
	}()

	return ch, nil
}

func (c *Calcium) replaceAndRemove(
	ctx context.Context,
	oldContainer *types.Container,
	opts *types.DeployOptions,
	index int) (*types.CreateContainerMessage, error) {
	var err error

	// 锁住，防止删除
	lock, err := c.Lock(ctx, fmt.Sprintf("rmcontainer_%s", oldContainer.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return nil, err
	}
	defer lock.Unlock(ctx)

	// 确保得到锁的时候容器没被干掉
	_, err = oldContainer.Inspect(ctx)
	if err != nil {
		return nil, err
	}

	// 停掉老的
	if err = c.stopOneContainer(ctx, oldContainer); err != nil {
		return nil, err
	}

	// 强制覆盖这个新容器为这个老容器的资源
	// TODO 这里并发不能可能需要在上层拷贝 opts
	opts.Memory = oldContainer.Memory
	opts.CPUQuota = oldContainer.Quota

	// 创建新容器，复用资源，如果失败会被自动回收，但是这里要重启老容器
	// 实际上会从 node 的抽象中减掉这部分的资源，因此资源计数器可能不准确，如果成功了，remove 老容器即可恢复
	createMessage := c.createAndStartContainer(ctx, index, oldContainer.Node, opts, oldContainer.CPU)
	if createMessage.Error != nil {
		// 重启容器, 并不关心是否启动成功
		if err = oldContainer.Engine.ContainerStart(ctx, oldContainer.ID, enginetypes.ContainerStartOptions{}); err != nil {
			log.Errorf("[replaceAndRemove] Old container %s restart failed %v", oldContainer.ID, err)
		}
		return nil, createMessage.Error
	}

	// 这里横竖会保证资源回收, 因此即便 remove 失败我们只需要考虑新容器占据了准确的资源配额即可
	if err = c.removeAndCleanOneContainer(ctx, oldContainer); err != nil {
		return nil, err
	}

	return createMessage, nil
}
