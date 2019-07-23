package calcium

import (
	"bufio"
	"context"
	"fmt"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ExecuteContainer executes commands in running containers
func (c *Calcium) ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions) (ch chan *types.ExecuteContainerMessage) {
	ch = make(chan *types.ExecuteContainerMessage)

	go func() {
		defer close(ch)

		var err error
		responses := []string{}
		defer func() {
			for _, resp := range responses {
				msg := &types.ExecuteContainerMessage{ContainerID: opts.ContainerID, Data: []byte(resp)}
				ch <- msg
			}
		}()

		container, err := c.GetContainer(ctx, opts.ContainerID)
		if err != nil {
			responses = append(responses, err.Error())
			return
		}

		info, err := container.Inspect(ctx)
		if err != nil {
			responses = append(responses, err.Error())
			return
		}

		if !info.Running {
			responses = append(responses, fmt.Sprintf("container %s not running, abort", opts.ContainerID))
			return
		}

		execConfig := &enginetypes.ExecConfig{
			Env:          opts.Envs,
			WorkingDir:   opts.Workdir,
			Cmd:          opts.Commands,
			AttachStderr: true,
			AttachStdout: true,
			AttachStdin:  false, // TODO
			Tty:          false, // TODO
			Detach:       false,
		}
		execID, err := container.Engine.ExecCreate(ctx, opts.ContainerID, execConfig)
		if err != nil {
			return
		}

		tty := false
		stdout, err := container.Engine.ExecAttach(ctx, execID, false, tty)
		if err != nil {
			return
		}

		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			data := scanner.Bytes()
			ch <- &types.ExecuteContainerMessage{ContainerID: opts.ContainerID, Data: data}
		}

		if err = scanner.Err(); err != nil {
			if err == context.Canceled {
				err = nil
				return
			}

			log.Errorf("[Calcium.ExecuteContainer] failed to parse log for %s: %v", opts.ContainerID, err)
			responses = append(responses, err.Error())
			return
		}

		log.Infof("[Calcium.ExecuteContainer] container exec complete: %s", opts.ContainerID)
	}()

	return
}
