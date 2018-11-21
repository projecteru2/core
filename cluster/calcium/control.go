package calcium

import (
	"context"
	"fmt"
	"sync"

	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/lock"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ControlContainer control containers status
func (c *Calcium) ControlContainer(ctx context.Context, IDs []string, t string) (chan *types.ControlContainerMessage, error) {
	ch := make(chan *types.ControlContainerMessage)

	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		for _, ID := range IDs {
			container, err := c.GetContainer(ctx, ID)
			if err != nil {
				ch <- &types.ControlContainerMessage{
					ContainerID: ID,
					Error:       err,
				}
				continue
			}

			wg.Add(1)
			go func(container *types.Container) {
				defer wg.Done()
				var err error
				defer func() {
					ch <- &types.ControlContainerMessage{
						ContainerID: container.ID,
						Error:       err,
					}
				}()

				containerJSON, err := container.Inspect(ctx)
				if err != nil {
					return
				}

				var message string
				switch t {
				case cluster.ContainerStop:
					message, err = c.doStopContainer(ctx, container, containerJSON, nil, false)
					log.Infof("[ControlContainer] Stop container %s output %s", container.ID, message)
					return
				case cluster.ContainerStart:
					message, err = c.doStartContainer(ctx, container, containerJSON)
					log.Infof("[ControlContainer] Start container %s output %s", container.ID, message)
					return
				case cluster.ContainerRestart:
					message, err = c.doStopContainer(ctx, container, containerJSON, nil, false)
					if err != nil {
						return
					}
					m2, e2 := c.doStartContainer(ctx, container, containerJSON)
					message += m2
					if e2 != nil {
						err = e2
					}
					log.Infof("[ControlContainer] Restart container %s output %s", container.ID, message)
					return
				default:
					err = types.ErrUnknownControlType
				}
			}(container)
		}
		wg.Wait()
	}()

	return ch, nil
}

func (c *Calcium) doStartContainer(ctx context.Context, container *types.Container, containerJSON enginetypes.ContainerJSON) (string, error) {
	var message string
	var err error

	if err = container.Start(ctx); err != nil {
		return message, err
	}

	// TODO healthcheck
	if container.Hook != nil && len(container.Hook.AfterStart) > 0 {
		var output []byte
		output, err = c.doContainerAfterStartHook(
			ctx, container,
			containerJSON.Config.User,
			containerJSON.Config.Env,
			container.Privileged,
		)
		message = string(output)
	}
	return message, err
}

func (c *Calcium) doStopContainer(ctx context.Context, container *types.Container, containerJSON enginetypes.ContainerJSON, ib *imageBucket, force bool) (string, error) {
	// 记录镜像
	if ib != nil {
		ib.Add(container.Podname, containerJSON.Config.Image)
	}

	var message string
	var err error
	if container.Hook != nil && len(container.Hook.BeforeStop) > 0 && containerJSON.Config != nil {
		output, err := c.doContainerBeforeStopHook(
			ctx, container,
			containerJSON.Config.User,
			containerJSON.Config.Env,
			container.Privileged, force,
		)
		message = string(output)
		if err != nil {
			return message, err
		}
	}

	if err = container.Stop(ctx, c.config.GlobalTimeout); err != nil {
		message += err.Error()
	}
	return message, err
}

func (c *Calcium) doRemoveContainer(ctx context.Context, container *types.Container) error {
	if err := container.Remove(ctx); err != nil {
		return err
	}

	return c.store.RemoveContainer(ctx, container)
}

func (c *Calcium) doLockContainer(ctx context.Context, container *types.Container) (*types.Container, enginetypes.ContainerJSON, lock.DistributedLock, error) {
	lock, err := c.Lock(ctx, fmt.Sprintf(cluster.ContainerLock, container.ID), int(c.config.GlobalTimeout.Seconds()))
	if err != nil {
		return container, enginetypes.ContainerJSON{}, nil, err
	}
	log.Debugf("[doLockContainer] Container %s locked", container.ID)
	// 确保是有这个容器的
	containerJSON, err := container.Inspect(ctx)
	if err != nil {
		lock.Unlock(ctx)
		return container, enginetypes.ContainerJSON{}, nil, err
	}
	// 更新容器元信息
	// 可能在等锁的过程中被 realloc 了
	rContainer, err := c.store.GetContainer(ctx, container.ID)
	if err != nil {
		lock.Unlock(ctx)
		return container, enginetypes.ContainerJSON{}, nil, err
	}
	return rContainer, containerJSON, lock, nil
}
