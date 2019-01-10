package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/cluster"
	enginetypes "github.com/projecteru2/core/engine/types"
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

				containerInfo, err := container.Inspect(ctx)
				if err != nil {
					return
				}

				var message string
				switch t {
				case cluster.ContainerStop:
					message, err = c.doStopContainer(ctx, container, containerInfo, nil, false)
					log.Infof("[ControlContainer] Stop container %s output %s", container.ID, message)
					return
				case cluster.ContainerStart:
					message, err = c.doStartContainer(ctx, container, containerInfo)
					log.Infof("[ControlContainer] Start container %s output %s", container.ID, message)
					return
				case cluster.ContainerRestart:
					message, err = c.doStopContainer(ctx, container, containerInfo, nil, false)
					if err != nil {
						return
					}
					m2, e2 := c.doStartContainer(ctx, container, containerInfo)
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

func (c *Calcium) doStartContainer(ctx context.Context, container *types.Container, containerInfo *enginetypes.VirtualizationInfo) (string, error) {
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
			containerInfo.User,
			containerInfo.Env,
			container.Privileged,
		)
		message = string(output)
	}
	return message, err
}

func (c *Calcium) doStopContainer(ctx context.Context, container *types.Container, containerInfo *enginetypes.VirtualizationInfo, ib *imageBucket, force bool) (string, error) {
	// 记录镜像
	if ib != nil {
		ib.Add(container.Podname, containerInfo.Image)
	}

	var message string
	var err error
	if container.Hook != nil && len(container.Hook.BeforeStop) > 0 {
		output, err := c.doContainerBeforeStopHook(
			ctx, container,
			containerInfo.User,
			containerInfo.Env,
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
