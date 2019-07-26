package calcium

import (
	"context"
	"fmt"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/engine"
	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

//RunAndWait implement lambda
func (c *Calcium) RunAndWait(ctx context.Context, opts *types.DeployOptions, stdinCh <-chan []byte) (chan *types.RunAndWaitMessage, error) {
	ch := make(chan *types.RunAndWaitMessage)

	// 强制为 json-file 输出
	opts.Entrypoint.Log = &types.LogConfig{Type: "json-file"}

	// count = 1 && OpenStdin
	if opts.OpenStdin && (opts.Count != 1 || opts.DeployMethod != cluster.DeployAuto) {
		close(ch)
		log.Errorf("Count %d method %s", opts.Count, opts.DeployMethod)
		return ch, types.ErrRunAndWaitCountOneWithStdin
	}

	// 创建容器, 有问题就
	// 不能让 CTX 作祟
	createChan, err := c.CreateContainer(context.Background(), opts)
	if err != nil {
		close(ch)
		log.Errorf("[RunAndWait] Create container error %s", err)
		return ch, err
	}

	// 来个goroutine处理剩下的事情
	// 基本上就是, attach拿日志写到channel, 以及等待容器结束后清理资源
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}

		for message := range createChan {
			// 可能不成功, 可能没有容器id, 但是其实第一项足够判断了
			// 不成功就无视掉
			if !message.Success || message.ContainerID == "" {
				log.Errorf("[RunAndWait] Create container error, %s", message.Error)
				continue
			}

			// 找不到对应node也不管
			// 理论上不会这样
			node, err := c.GetNode(ctx, message.Podname, message.Nodename)
			if err != nil {
				log.Errorf("[RunAndWait] Can't find node, %v", err)
				continue
			}

			// 加个task
			wg.Add(1)

			// goroutine logs日志然后处理了写回给channel
			// 日志跟task无关, 不管wg
			go func(node *types.Node, containerID string) {
				defer wg.Done()
				defer log.Infof("[RunAndWait] Container %s finished and removed", utils.ShortID(containerID))
				// CONTEXT 这里的不应该受到 client 的影响
				defer c.doRemoveContainerSync(context.Background(), []string{containerID})

				stdoutCh := make(chan []byte)
				errCh := make(chan error)

				if opts.OpenStdin {
					attachOpt := &enginetypes.VirtualizationAttachOption{
						AttachStdin:  stdinCh,
						AttachStdout: stdoutCh,
						Errors:       errCh,
					}
					if err = node.Engine.VirtualizationAttach(ctx, containerID, true, true, true, true, attachOpt); err != nil {
						log.Errorf("[RunAndWait] Failed to attach container %s, %v", containerID, err)
						return
					}

				} else {
					if err = engine.VirtualizationLogToChan(ctx, node.Engine, containerID, true, true, true, stdoutCh, errCh); err != nil {
						log.Errorf("[RunAndWait] failed to get logs for %s: %v", containerID, err)
						return
					}
				}

				for {
					select {
					case err := <-errCh:
						if err != nil {
							log.Errorf("[RunAndWait] failed to parse log: %v", err)
							ch <- &types.RunAndWaitMessage{ContainerID: containerID, Data: []byte(fmt.Sprintf("[exitcode] unknown %v", err))}
							return
						}
					case data, ok := <-stdoutCh:
						ch <- &types.RunAndWaitMessage{
							ContainerID: containerID,
							Data:        data,
						}
						log.Debugf("[RunAndWait] %s output: %s", utils.ShortID(containerID), data)
						if !ok {
							return
						}
					}
				}

			}(node, message.ContainerID)
		}

		// 等待全部任务完成才可以关闭channel
		wg.Wait()
		log.Info("[RunAndWait] Finish run and wait for containers")
	}()

	return ch, nil
}
