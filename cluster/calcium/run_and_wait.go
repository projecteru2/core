package calcium

import (
	"bufio"
	"fmt"

	log "github.com/Sirupsen/logrus"
	enginetypes "github.com/docker/docker/api/types"
	"gitlab.ricebook.net/platform/core/types"
	"golang.org/x/net/context"
)

func (c *calcium) RunAndWait(specs types.Specs, opts *types.DeployOptions) (chan *types.RunAndWaitMessage, error) {
	ch := make(chan *types.RunAndWaitMessage)
	if opts.Count != 1 {
		return ch, fmt.Errorf("Only one container can be run and wait")
	}

	createChan, err := c.CreateContainer(specs, opts)
	if err != nil {
		log.Errorf("[RunAndWait] Create container error, %s", err.Error())
		return ch, err
	}

	message := &types.CreateContainerMessage{}
	for m := range createChan {
		message = m
	}

	if message.ContainerID == "" {
		log.Errorf("[RunAndWait] Can't find container id")
		return ch, fmt.Errorf("Error during create container")
	}

	node, err := c.store.GetNode(message.Podname, message.Nodename)
	if err != nil {
		log.Errorf("[RunAndWait] Can't find node, %s", err.Error())
		return ch, err
	}

	resp, err := node.Engine.ContainerAttach(context.Background(), message.ContainerID, enginetypes.ContainerAttachOptions{Stream: true, Stdout: true, Stderr: true})
	if err != nil {
		log.Errorf("[RunAndWait] Failed to attach container %s, %s", message.ContainerID, err.Error())
		return ch, err
	}

	go func() {
		scanner := bufio.NewScanner(resp.Reader)
		defer close(ch)
		log.Infof("[RunAndWait] Container %s attached, send logs", message.ContainerID)
		for scanner.Scan() {
			m := &types.RunAndWaitMessage{ContainerID: message.ContainerID, Data: scanner.Text()}
			ch <- m
		}
		log.Infof("[RunAndWait] Container %s finished, remove", message.ContainerID)
		c.RemoveContainer([]string{message.ContainerID})
	}()

	return ch, nil
}
