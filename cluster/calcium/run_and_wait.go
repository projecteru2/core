package calcium

import (
	"bufio"
	"fmt"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"gitlab.ricebook.net/platform/core/types"
	"gitlab.ricebook.net/platform/core/utils"
	"golang.org/x/net/context"
)

func (c *calcium) RunAndWait(specs types.Specs, opts *types.DeployOptions) (chan *types.RunAndWaitMessage, error) {
	ch := make(chan *types.RunAndWaitMessage)

	// 强制为 json-file 输出
	entry, _ := specs.Entrypoints[opts.Entrypoint]
	entry.LogConfig = "json-file"
	specs.Entrypoints[opts.Entrypoint] = entry

	// 默认给出1200秒的超时时间吧
	// 没别的地方好传了, 不如放这里好了, 不需要用的就默认0或者不传
	waitTimeout := entry.RunAndWaitTimeout
	if waitTimeout == 0 {
		waitTimeout = c.config.RunAndWaitTimeout
	}

	// 创建容器, 有问题就gg
	createChan, err := c.CreateContainer(specs, opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create container error, %v", err)
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

			// goroutine attach日志然后处理了写回给channel
			// 日志跟task无关, 不管wg
			go func(node *types.Node, containerID string) {
				defer wg.Done()
				defer log.Infof("[RunAndWait] Container %s finished and removed", containerID[:12])
				defer c.removeContainerSync([]string{containerID})

				ctx, _ := context.WithTimeout(context.Background(), time.Duration(waitTimeout)*time.Second)
				resp, err := node.Engine.ContainerLogs(ctx, containerID, logsOpts)
				if err != nil {
					log.Errorf("[RunAndWait] Failed to get logs, %v", err)
					return
				}

				stream := utils.FuckDockerStream(resp)
				scanner := bufio.NewScanner(stream)
				for scanner.Scan() {
					data := scanner.Bytes()
					ch <- &types.RunAndWaitMessage{ContainerID: containerID, Data: data}
					log.Debugf("[RunAndWait] %s %s", containerID[:12], data)
				}

				if err := scanner.Err(); err != nil {
					log.Errorf("[RunAndWait] Parse log failed, %v", err)
					return
				}

				code, err := node.Engine.ContainerWait(context.Background(), containerID)
				exitData := []byte(fmt.Sprintf("[exitcode] %d", code))
				if err != nil {
					log.Errorf("%s run failed, %v", containerID[:12], err)
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
