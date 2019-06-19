package calcium

import (
	"bufio"
	"context"

	"github.com/projecteru2/core/types"
)

// LogStream log stream for one container
func (c *Calcium) LogStream(ctx context.Context, ID string) (chan *types.LogStreamMessage, error) {
	ch := make(chan *types.LogStreamMessage)
	go func() {
		defer close(ch)
		container, err := c.store.GetContainer(ctx, ID)
		if err != nil {
			ch <- &types.LogStreamMessage{ID: ID, Error: err}
			return
		}

		resp, err := container.Engine.VirtualizationLogs(ctx, ID, true, true, true)
		if err != nil {
			ch <- &types.LogStreamMessage{ID: ID, Error: err}
			return
		}

		scanner := bufio.NewScanner(resp)
		for scanner.Scan() {
			data := scanner.Bytes()
			ch <- &types.LogStreamMessage{ID: ID, Data: data}
		}
	}()
	return ch, nil
}
