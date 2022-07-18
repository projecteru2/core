package calcium

import (
	"bufio"
	"context"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// LogStream log stream for one workload
func (c *Calcium) LogStream(ctx context.Context, opts *types.LogStreamOptions) (chan *types.LogStreamMessage, error) {
	logger := log.WithField("Calcium", "LogStream").WithField("opts", opts)
	ch := make(chan *types.LogStreamMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		workload, err := c.GetWorkload(ctx, opts.ID)
		if err != nil {
			ch <- &types.LogStreamMessage{ID: opts.ID, Error: logger.ErrWithTracing(ctx, err)}
			return
		}

		stdout, stderr, err := workload.Engine.VirtualizationLogs(ctx, &enginetypes.VirtualizationLogStreamOptions{
			ID:     opts.ID,
			Tail:   opts.Tail,
			Since:  opts.Since,
			Until:  opts.Until,
			Follow: opts.Follow,
			Stdout: true,
			Stderr: true,
		})
		if err != nil {
			ch <- &types.LogStreamMessage{ID: opts.ID, Error: logger.ErrWithTracing(ctx, err)}
			return
		}

		for m := range processStdStream(ctx, stdout, stderr, bufio.ScanLines, byte('\n')) {
			ch <- &types.LogStreamMessage{ID: opts.ID, Data: m.Data, StdStreamType: m.StdStreamType}
		}
	})

	return ch, nil
}
