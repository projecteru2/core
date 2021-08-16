package calcium

import (
	"bufio"
	"context"
	"github.com/pkg/errors"

	enginetypes "github.com/projecteru2/core/engine/types"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// LogStream log stream for one workload
func (c *Calcium) LogStream(ctx context.Context, opts *types.LogStreamOptions) (chan *types.LogStreamMessage, error) {
	logger := log.WithField("Calcium", "LogStream").WithField("opts", opts)
	ch := make(chan *types.LogStreamMessage)
	utils.SentryGo(func() {
		defer close(ch)
		workload, err := c.GetWorkload(ctx, opts.ID)
		if err != nil {
			ch <- &types.LogStreamMessage{ID: opts.ID, Error: logger.Err(ctx, err)}
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
		if err != nil && !errors.Is(err, types.ErrFunctionNotImplemented("VirtualizationLogs")) {
			ch <- &types.LogStreamMessage{ID: opts.ID, Error: logger.Err(ctx, err)}
			return
		}

		for m := range processStdStream(ctx, stdout, stderr, bufio.ScanLines, byte('\n')) {
			ch <- &types.LogStreamMessage{ID: opts.ID, Data: m.Data, StdStreamType: m.StdStreamType}
		}
	})

	return ch, nil
}
