package calcium

import (
	"bufio"
	"fmt"
	"sync"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

// FUCK DOCKER
const PREFIXLEN int = 8

func (c *calcium) RunAndWait(specs types.Specs, opts *types.DeployOptions) (chan *types.RunAndWaitMessage, error) {
	ch := make(chan *types.RunAndWaitMessage)

	// 强制为 json-file 输出
	entry, _ := specs.Entrypoints[opts.Entrypoint]
	entry.LogConfig = "json-file"
	specs.Entrypoints[opts.Entrypoint] = entry

	createChan, err := c.CreateContainer(specs, opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create container error, %s", err.Error())
		return ch, err
	}

	go func() {
		wg := &sync.WaitGroup{}
		defer log.Info("[RunAndWait] Finish run and wait for containers")
		defer close(ch)
		defer wg.Wait()
		logsOpts := enginetypes.ContainerLogsOptions{Follow: true, ShowStdout: true, ShowStderr: true}

		for message := range createChan {
			wg.Add(1)
			if message.ContainerID == "" {
				log.Errorf("[RunAndWait] Can't find container id %s", err.Error())
				continue
			}

			node, err := c.store.GetNode(message.Podname, message.Nodename)
			if err != nil {
				log.Errorf("[RunAndWait] Can't find node, %s", err.Error())
				continue
			}

			go func(node *types.Node, message *types.CreateContainerMessage) {
				defer wg.Done()
				resp, err := node.Engine.ContainerLogs(context.Background(), message.ContainerID, logsOpts)
				if err != nil {
					data := fmt.Sprintf("[RunAndWait] Failed to get logs, %s", err.Error())
					ch <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: data}
					return
				}

				scanner := bufio.NewScanner(resp)
				for scanner.Scan() {
					data := scanner.Bytes()[PREFIXLEN:]
					log.Debugf("[RunAndWait] %s %s", message.ContainerID[:12], data)
					m := &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: string(data)}
					ch <- m
				}

				if err := scanner.Err(); err != nil {
					data := fmt.Sprintf("[RunAndWait] Parse log failed, %s", err.Error())
					ch <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: data}
					return
				}

				container, err := c.GetContainer(message.ContainerID)
				if err != nil {
					data := fmt.Sprintf("[RunAndWait] Container not found, %s", err.Error())
					ch <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: data}
					return
				}

				containerJSON, err := container.Inspect()
				defer c.removeOneContainer(container, containerJSON)
				if err == nil {
					ch <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: fmt.Sprintf("[exitcode] %d", containerJSON.State.ExitCode)}
				} else {
					ch <- &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: fmt.Sprintf("[exitcode]unknown %s", err.Error())}
				}
				log.Infof("[RunAndWait] Container %s finished, remove", message.ContainerID)
			}(node, message)
		}
	}()

	return ch, nil
}
