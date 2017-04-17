package calcium

import (
	"bufio"
	"fmt"

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

	createChan, err := c.CreateContainer(specs, opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create container error, %s", err.Error())
		return ch, err
	}

	go func() {
		defer log.Info("[RunAndWait] Finish run and wait for containers")
		defer close(ch)
		logsOpts := enginetypes.ContainerLogsOptions{Follow: true, ShowStdout: true, ShowStderr: true}

		ids := map[string]*types.Node{}
		for message := range createChan {
			if message.ContainerID == "" {
				log.Errorf("[RunAndWait] Can't find container id %s", err.Error())
				continue
			}

			node, err := c.store.GetNode(message.Podname, message.Nodename)
			if err != nil {
				log.Errorf("[RunAndWait] Can't find node, %s", err.Error())
				continue
			}

			ids[message.ContainerID] = node
			go func(node *types.Node, containerID string) {
				resp, err := node.Engine.ContainerLogs(context.Background(), containerID, logsOpts)
				if err != nil {
					log.Errorf("[RunAndWait] Failed to get logs, %s", err.Error())
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
					log.Errorf("[RunAndWait] Parse log failed, %s", err.Error())
					return
				}
			}(node, message.ContainerID)
		}

		rmids := []string{}
		for id, node := range ids {
			rmids = append(rmids, id)
			code, err := node.Engine.ContainerWait(context.Background(), id)
			exitData := []byte(fmt.Sprintf("[exitcode] %d", code))
			if err != nil {
				log.Errorf("%s run failed, %s", id[:12], err.Error())
				exitData = []byte(fmt.Sprintf("[exitcode]unknown %s", err.Error()))
			}
			ch <- &types.RunAndWaitMessage{ContainerID: id, Data: exitData}
			log.Infof("[RunAndWait] Container %s finished, remove", id[:12])
		}
		go c.RemoveContainer(rmids)
	}()

	return ch, nil
}
