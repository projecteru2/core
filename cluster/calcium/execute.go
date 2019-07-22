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
			errMsg = fmt.Sprintf("[Calcium.ExecuteContainer] failed to get container %s: %v", opts.ContainerID, err)
			return
		}

		info := &enginetypes.VirtualizationInfo{}
		if info, err = container.Inspect(ctx); err != nil {
			errMsg = fmt.Sprintf("[Calcium.ExecuteContainer] failed to get info of contaienr %s: %v", opts.ContainerID, err)
			return
		}

		if !info.Running {
			errMsg = fmt.Sprintf("[Calcium.ExecuteContainer] container %s not running, abort", opts.ContainerID)
			return
		}

		node := &types.Node{}
		if node, err = c.GetNode(ctx, container.Podname, container.Nodename); err != nil {
			errMsg = fmt.Sprintf("[Calcium.ExecuteContainer] failed to get node for container %s: %v", container.ID, err)
			return
		}

		var output io.ReadCloser
		if _, output, err = node.Engine.VirtualizationExecute(ctx, opts.ContainerID, opts.Commands, opts.Envs, opts.Workdir); err != nil {
			errMsg = fmt.Sprintf("[Calcium.ExecuteContainer] failed to execute container %s: %v", container.ID, err)
			return
		}

		scanner := bufio.NewScanner(output)
		for scanner.Scan() {
			data := scanner.Bytes()
			ch <- &types.ExecuteContainerMessage{ContainerID: opts.ContainerID, Data: data}
		}

		if err = scanner.Err(); err != nil {
			if err == context.Canceled {
				return
			}

			log.Errorf("[Calcium.ExecuteContainer] failed to parse log for %s: %v", opts.ContainerID, err)
			errMsg = fmt.Sprintf("[exitcde] unknown error: %v", err)
			return
		}

		log.Infof("[Calcium.ExecuteContainer] container exec complete: %s", opts.ContainerID)
	}()

	return
}
