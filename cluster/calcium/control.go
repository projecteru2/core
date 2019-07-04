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
func (c *Calcium) ControlContainer(ctx context.Context, IDs []string, t string, force bool) (chan *types.ControlContainerMessage, error) {
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
						log.Info("[ControlContainer] Output:")
						log.Info(string(types.HookOutput(message)))
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
					message, err = c.doStopContainer(ctx, container, containerInfo, force)
					return
				case cluster.ContainerStart:
					message, err = c.doStartContainer(ctx, container, containerInfo, force)
					return
				case cluster.ContainerRestart:
					if containerInfo, err := container.Inspect(ctx); err == nil && containerInfo.Running {
						message, err = c.doStopContainer(ctx, container, containerInfo, force)
						if err != nil {
							return
						}

					} else if err != nil {
						log.Errorf("[ControlContainer] Inspect container %s failed %v", container.ID, err)
					} else if !containerInfo.Running {
						message = append(message, bytes.NewBufferString("container stopped, can't run hook\n"))
					}
					m2, e2 := c.doStartContainer(ctx, container, containerInfo, force)
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

func (c *Calcium) doStartContainer(ctx context.Context, container *types.Container, containerInfo *enginetypes.VirtualizationInfo, force bool) ([]*bytes.Buffer, error) {
	var message []*bytes.Buffer
	var err error

	startCtx, cancel := context.WithTimeout(ctx, c.config.GlobalTimeout)
	defer cancel()
	if err = container.Start(startCtx); err != nil {
		return message, err
	}
	// TODO healthcheck
	if container.Hook != nil && len(container.Hook.AfterStart) > 0 {
		message, err = c.doHook(
			ctx,
			container.ID, containerInfo.User,
			container.Hook.AfterStart, containerInfo.Env,
			container.Hook.Force, container.Privileged,
			force, container.Engine,
		)
	}
	return message, err
}

func (c *Calcium) doStopContainer(ctx context.Context, container *types.Container, containerInfo *enginetypes.VirtualizationInfo, force bool) ([]*bytes.Buffer, error) {
	var message []*bytes.Buffer
	var err error

	if container.Hook != nil && len(container.Hook.BeforeStop) > 0 {
		message, err = c.doHook(
			ctx,
			container.ID, containerInfo.User,
			container.Hook.BeforeStop, containerInfo.Env,
			container.Hook.Force, container.Privileged,
			force, container.Engine,
		)
		if err != nil {
			return message, err
		}
	}

	// 这里 block 的问题很严重，按照目前的配置是 5 分钟一级的 block
	// 一个简单的处理方法是相信 ctx 不相信 engine 自身的处理
	// 另外我怀疑 engine 自己的 timeout 实现是完全的等 timeout 而非结束了就退出
	stopCtx, cancel := context.WithTimeout(ctx, c.config.GlobalTimeout)
	defer cancel()
	if err = container.Stop(stopCtx); err != nil {
		message = append(message, bytes.NewBufferString(err.Error()))
	}
	return message, err
}
