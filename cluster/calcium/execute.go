package calcium

import (
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// ExecuteContainer executes commands in running containers
func (c *Calcium) ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions, inCh <-chan []byte) (ch chan *types.ExecuteContainerMessage) {
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

		execConfig := &enginetypes.ExecConfig{
			Env:          opts.Envs,
			WorkingDir:   opts.Workdir,
			Cmd:          opts.Commands,
			AttachStderr: true,
			AttachStdout: true,
			AttachStdin:  opts.OpenStdin,
			Tty:          opts.OpenStdin,
			Detach:       false,
		}
		execID, err := container.Engine.ExecCreate(ctx, opts.ContainerID, execConfig)
		if err != nil {
			log.Errorf("[Calcium.ExecuteContainer] failed to create execID: %v", err)
			return
		}

		outStream, inStream, err := container.Engine.ExecAttach(ctx, execID)
		if err != nil {
			log.Errorf("[Calcium.ExecContainer] failed to attach execID: %v", err)
			return
		}

		if opts.OpenStdin {
			ProcessVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return container.Engine.VirtualizationResize(ctx, container.ID, height, width)
			})
		}

		for data := range ProcessVirtualizationOutStream(ctx, outStream) {
			ch <- &types.ExecuteContainerMessage{ContainerID: opts.ContainerID, Data: data}
		}

		log.Infof("[Calcium.ExecuteContainer] container exec complete: %s", opts.ContainerID)
	}()

	return
}
