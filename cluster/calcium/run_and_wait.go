package calcium

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

func (c *calcium) RunAndWait(specs types.Specs, opts *types.DeployOptions, stdin io.ReadCloser) (chan *types.RunAndWaitMessage, error) {
	ch := make(chan *types.RunAndWaitMessage)

	// 强制为 json-file 输出
	entry, _ := specs.Entrypoints[opts.Entrypoint]
	entry.LogConfig = "json-file"
	specs.Entrypoints[opts.Entrypoint] = entry

	waitTimeout := c.config.GlobalTimeout
	if entry.RunAndWaitTimeout > 0 {
		waitTimeout = time.Duration(entry.RunAndWaitTimeout) * time.Second
	}

	// count = 1 && OpenStdin
	if opts.OpenStdin && opts.Count != 1 {
		close(ch)
		err := fmt.Errorf("[RunAndWait] Count must be 1 if OpenStdin is true, count is %d now", opts.Count)
		return ch, err
	}

	// 创建容器, 有问题就gg
	log.Debugf("[RunAndWait] Args: %v, %v, %v", waitTimeout, specs, opts)
	createChan, err := c.CreateContainer(specs, opts)
	if err != nil {
		close(ch)
		log.Errorf("[RunAndWait] Create container error %s", err)
		return ch, err
	}

	// 来个goroutine处理剩下的事情
	// 基本上就是, attach拿日志写到channel, 以及等待容器结束后清理资源
	go func() {
		defer close(ch)
		logsOpts := enginetypes.ContainerLogsOptions{Follow: true, ShowStdout: true, ShowStderr: true}
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
			node, err := c.store.GetNode(message.Podname, message.Nodename)
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
				defer log.Infof("[RunAndWait] Container %s finished and removed", containerID[:12])
				defer c.removeContainerSync([]string{containerID})

				ctx, cancel := context.WithTimeout(context.Background(), waitTimeout)
				defer cancel()
				resp, err := node.Engine.ContainerLogs(ctx, containerID, logsOpts)
				if err != nil {
					log.Errorf("[RunAndWait] Failed to get logs, %v", err)
					ch <- &types.RunAndWaitMessage{ContainerID: containerID,
						Data: []byte(fmt.Sprintf("[exitcode] unknown %v", err))}
					return
				}

				if opts.OpenStdin {
					go func() {
						r, err := node.Engine.ContainerAttach(
							context.Background(), containerID,
							enginetypes.ContainerAttachOptions{Stream: true, Stdin: true})
						defer r.Close()
						defer stdin.Close()
						if err != nil {
							log.Errorf("[RunAndWait] Failed to attach container, %v", err)
							return
						}
						io.Copy(r.Conn, stdin)
					}()
				}

				stream := utils.FuckDockerStream(resp)
				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					data := scanner.Bytes()
					ch <- &types.RunAndWaitMessage{
						ContainerID: containerID,
						Data:        data,
					}
					log.Debugf("[RunAndWait] %s output: %s", containerID[:12], data)
				}

				if err := scanner.Err(); err != nil {
					log.Errorf("[RunAndWait] Parse log failed, %v", err)
					ch <- &types.RunAndWaitMessage{ContainerID: containerID,
						Data: []byte(fmt.Sprintf("[exitcode] unknown %v", err))}
					return
				}

				// 超时的情况下根本不会到这里
				// 不超时的情况下这里肯定会立即返回
				code, err := node.Engine.ContainerWait(context.Background(), containerID)
				exitData := []byte(fmt.Sprintf("[exitcode] %d", code))
				if err != nil {
					log.Errorf("[RunAndWait] %s run failed, %v", containerID[:12], err)
					exitData = []byte(fmt.Sprintf("[exitcode] unknown %v", err))
				}
				ch <- &types.RunAndWaitMessage{ContainerID: containerID, Data: exitData}
			}(node, message.ContainerID)
		}

		// 等待全部任务完成才可以关闭channel
		wg.Wait()
		log.Info("[RunAndWait] Finish run and wait for containers")
	}()

	return ch, nil
}
