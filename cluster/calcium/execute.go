package calcium

import (
	"context"
	"fmt"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// ExecuteContainer executes commands in running containers
func (c *Calcium) ExecuteContainer(ctx context.Context, opts *types.ExecuteContainerOptions) (ch chan *types.ExecuteContainerMessage, err error) {
	ch = make(chan *types.ExecuteContainerMessage)
	go func() {
		defer close(ch)

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

		resp := &enginetypes.VirtualizationExecuteResult{}
		if resp, err = node.Engine.VirtualizationExecute(ctx, opts.ContainerID, opts.Commands, opts.Envs, opts.Workdir); err != nil {
			errMsg = fmt.Sprintf("[Calcium.ExecuteContainer] failed to execute container %s: %v", container.ID, err)
			return
		}

		ch <- &types.ExecuteContainerMessage{ContainerID: opts.ContainerID, Data: []byte(resp.Stdout)}

	}()
	return
}
