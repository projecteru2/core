package calcium

import (
	"bufio"
	"context"
	"fmt"
	"io"

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
		errMsg := ""
		defer func() {
			if err != nil && errMsg != "" {
				msg := &types.ExecuteContainerMessage{ContainerID: opts.ContainerID, Data: []byte(errMsg)}
				ch <- msg
			}
		}()

		container, err := c.GetContainer(ctx, opts.ContainerID)
		if err != nil {
			errMsg = fmt.Sprintf("failed to get container %s: %v", opts.ContainerID, err)
			return
		}

		info := &enginetypes.VirtualizationInfo{}
		if info, err = container.Inspect(ctx); err != nil {
			errMsg = fmt.Sprintf("failed to get info of contaienr %s: %v", opts.ContainerID, err)
			return
		}

		if !info.Running {
			errMsg = fmt.Sprintf("container %s not running, abort", opts.ContainerID)
			return
		}

		execID := ""
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
		if execID, err = container.Engine.ExecCreate(ctx, opts.ContainerID, execConfig); err != nil {
			return
		}

		tty := false
		var output io.ReadCloser
		if output, err = container.Engine.ExecAttach(ctx, execID, false, tty); err != nil {
			return
		}

		scanner := bufio.NewScanner(output)
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
			errMsg = fmt.Sprintf("unknown error: %v", err)
			return
		}

		log.Infof("[Calcium.ExecuteContainer] container exec complete: %s", opts.ContainerID)
	}()

	return
}
