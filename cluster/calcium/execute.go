package calcium

import (
	"context"
	"strconv"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
	log "github.com/sirupsen/logrus"
)

// ExecuteContainer executes commands in running containers
func (c *Calcium) ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions, inCh <-chan []byte) (ch chan *types.AttachContainerMessage) {
	ch = make(chan *types.AttachContainerMessage)

	go func() {
		defer close(ch)
		var err error
		responses := []string{}
		defer func() {
			for _, resp := range responses {
				msg := &types.AttachContainerMessage{ContainerID: opts.ContainerID, Data: []byte(resp)}
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
			log.Errorf("[ExecuteContainer] Failed to create execID: %v", err)
			return
		}

		outStream, inStream, err := container.Engine.ExecAttach(ctx, execID, execConfig.Tty)
		if err != nil {
			log.Errorf("[ExecuteContainer] Failed to attach execID: %v", err)
			return
		}

		if opts.OpenStdin {
			processVirtualizationInStream(ctx, inStream, inCh, func(height, width uint) error {
				return container.Engine.ExecResize(ctx, execID, height, width)
			})
		}

		for data := range processVirtualizationOutStream(ctx, outStream) {
			ch <- &types.AttachContainerMessage{ContainerID: opts.ContainerID, Data: data}
		}

		execCode, err := container.Engine.ExecExitCode(ctx, execID)
		if err != nil {
			log.Errorf("[ExecuteContainer] Failed to get exitcode: %v", err)
			return
		}

		exitData := []byte(exitDataPrefix + strconv.Itoa(execCode))
		ch <- &types.AttachContainerMessage{ContainerID: opts.ContainerID, Data: exitData}
		log.Infof("[ExecuteContainer] Execuate in container %s complete", utils.ShortID(opts.ContainerID))
	}()

	return
}
