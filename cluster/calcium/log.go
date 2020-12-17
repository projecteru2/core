package calcium

import (
	"bufio"
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/types"
)

// LogStream log stream for one workload
func (c *Calcium) LogStream(ctx context.Context, opts *types.LogStreamOptions) (chan *types.LogStreamMessage, error) {
	ch := make(chan *types.LogStreamMessage)
	go func() {
		defer close(ch)
		workload, err := c.GetWorkload(ctx, opts.ID)
		if err != nil {
			ch <- &types.LogStreamMessage{ID: opts.ID, Error: err}
			return
		}

		resp, err := workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
			ID:     opts.ID,
			Tail:   opts.Tail,
			Since:  opts.Since,
			Until:  opts.Until,
			Follow: opts.Follow,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			ch <- &types.LogStreamMessage{ID: opts.ID, Error: err}
			return
		}

		scanner := bufio.NewScanner(resp)
		for scanner.Scan() {
			data := scanner.Bytes()
			ch <- &types.LogStreamMessage{ID: opts.ID, Data: data}
		}
	}()
	return ch, nil
}
