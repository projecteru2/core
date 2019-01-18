package calcium

import (
	"bytes"
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
				var message []*bytes.Buffer
				defer func() {
					if err == nil {
						log.Infof("[ControlContainer] Control container %s %s", container.ID, t)
						log.Infof("[ControlContainer] Output:\n %s", string(types.HookOutput(message)))
					}
					ch <- &types.ControlContainerMessage{
						ContainerID: container.ID,
						Error:       err,
						Hook:        message,
					}
				}()

				containerInfo, err := container.Inspect(ctx)
				if err != nil {
					return
				}

				switch t {
				case cluster.ContainerStop:
					message, err = c.doStopContainer(ctx, container, containerInfo, nil, false)
					return
				case cluster.ContainerStart:
					message, err = c.doStartContainer(ctx, container, containerInfo)
					return
				case cluster.ContainerRestart:
					message, err = c.doStopContainer(ctx, container, containerInfo, nil, false)
					if err != nil {
						return
					}
					m2, e2 := c.doStartContainer(ctx, container, containerInfo)
					message = append(message, m2...)
					if e2 != nil {
						err = e2
					}
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

func (c *Calcium) doStartContainer(ctx context.Context, container *types.Container, containerInfo *enginetypes.VirtualizationInfo) ([]*bytes.Buffer, error) {
	var message []*bytes.Buffer
	var err error

	if err = container.Start(ctx); err != nil {
		return message, err
	}
	// TODO healthcheck
	if container.Hook != nil && len(container.Hook.AfterStart) > 0 {
		message, err = c.doHook(
			ctx,
			container.ID, containerInfo.User,
			container.Hook.AfterStart, containerInfo.Env,
			container.Hook.Force, false,
			container.Privileged,
			container.Engine,
		)
	}
	return message, err
}

func (c *Calcium) doStopContainer(ctx context.Context, container *types.Container, containerInfo *enginetypes.VirtualizationInfo, ib *imageBucket, force bool) ([]*bytes.Buffer, error) {
	var message []*bytes.Buffer
	var err error
	// 记录镜像
	if ib != nil {
		ib.Add(container.Podname, containerInfo.Image)
	}

	if container.Hook != nil && len(container.Hook.BeforeStop) > 0 {
		message, err = c.doHook(
			ctx,
			container.ID, containerInfo.User,
			container.Hook.BeforeStop, containerInfo.Env,
			container.Hook.Force, force,
			container.Privileged,
			container.Engine,
		)
		if err != nil {
			return message, err
		}
	}

	if err = container.Stop(ctx, c.config.GlobalTimeout); err != nil {
		message = append(message, bytes.NewBufferString(err.Error()))
	}
	return message, err
}
