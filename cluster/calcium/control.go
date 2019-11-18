package calcium

import (
	"bytes"
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/cluster"
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
			wg.Add(1)
			go func(ID string) {
				defer wg.Done()
				var message []*bytes.Buffer
				err := c.withContainerLocked(ctx, ID, func(container *types.Container, runtimeMeta *types.RuntimeMeta) error {
					if runtimeMeta == nil {
						return types.ErrRunningStatusUnknown
					}
					var err error
					switch t {
					case cluster.ContainerStop:
						message, err = c.doStopContainer(ctx, container, runtimeMeta, force)
						return err
					case cluster.ContainerStart:
						message, err = c.doStartContainer(ctx, container, runtimeMeta, force)
						return err
					case cluster.ContainerRestart:
						message, err = c.doStopContainer(ctx, container, runtimeMeta, force)
						if err != nil {
							return err
						}
						runtimeMeta.Running = false
						m2, e2 := c.doStartContainer(ctx, container, runtimeMeta, force)
						message = append(message, m2...)
						if e2 != nil {
							return fmt.Errorf("%w", e2)
						}
						return nil
					}
					return types.ErrUnknownControlType
				})
				if err == nil {
					log.Infof("[ControlContainer] Container %s %s", ID, t)
					log.Info("[ControlContainer] Hook Output:")
					log.Info(string(types.HookOutput(message)))
				}
				ch <- &types.ControlContainerMessage{
					ContainerID: ID,
					Error:       err,
					Hook:        message,
				}
			}(ID)
		}
		wg.Wait()
	}()

	return ch, nil
}

func (c *Calcium) doStartContainer(ctx context.Context, container *types.Container, runtimeMeta *types.RuntimeMeta, force bool) ([]*bytes.Buffer, error) {
	var message []*bytes.Buffer
	if runtimeMeta.Running {
		message = append(message, bytes.NewBufferString("container already running, can't run hook\n"))
		return message, nil
	}
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
			container.ID, container.User,
			container.Hook.AfterStart, container.Env,
			container.Hook.Force, container.Privileged,
			force, container.Engine,
		)
	}
	return message, err
}

func (c *Calcium) doStopContainer(ctx context.Context, container *types.Container, runtimeMeta *types.RuntimeMeta, force bool) ([]*bytes.Buffer, error) {
	var message []*bytes.Buffer
	if !runtimeMeta.Running {
		message = append(message, bytes.NewBufferString("container stopped, can't run hook\n"))
		return message, nil
	}
	var err error

	if container.Hook != nil && len(container.Hook.BeforeStop) > 0 {
		message, err = c.doHook(
			ctx,
			container.ID, container.User,
			container.Hook.BeforeStop, container.Env,
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
