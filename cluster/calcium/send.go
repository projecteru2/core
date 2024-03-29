package calcium

import (
	"bytes"
	"context"
	"sync"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Send files to workload
func (c *Calcium) Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error) {
	logger := log.WithFunc("calcium.Send").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}
	ch := make(chan *types.SendMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}
		wg.Add(len(opts.IDs))

		for _, ID := range opts.IDs {
			logger.Infof(ctx, "Send files to %s", ID)
			_ = c.pool.Invoke(func(ID string) func() {
				return func() {
					defer wg.Done()
					if err := c.withWorkloadLocked(ctx, ID, false, func(ctx context.Context, workload *types.Workload) error {
						for _, file := range opts.Files {
							err := c.doSendFileToWorkload(ctx, workload.Engine, workload.ID, file)
							logger.Error(ctx, err)
							ch <- &types.SendMessage{ID: ID, Path: file.Filename, Error: err}
						}
						return nil
					}); err != nil {
						logger.Error(ctx, err)
						ch <- &types.SendMessage{ID: ID, Error: err}
					}
				}
			}(ID))
		}
		wg.Wait()
	})
	return ch, nil
}

func (c *Calcium) doSendFileToWorkload(ctx context.Context, engine engine.API, ID string, file types.LinuxFile) error {
	log.WithFunc("calcium.doSendFileToWorkload").Infof(ctx, "Send file to %s:%s", ID, file.Filename)
	return engine.VirtualizationCopyChunkTo(ctx, ID, file.Filename, int64(len(file.Content)), bytes.NewReader(file.Clone().Content), file.UID, file.GID, file.Mode)
}
