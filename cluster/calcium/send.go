package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"

	"github.com/pkg/errors"
)

// Send files to workload
func (c *Calcium) Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error) {
	logger := log.WithField("Calcium", "Send").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.ErrWithTracing(ctx, errors.WithStack(err))
	}
	ch := make(chan *types.SendMessage)
	utils.SentryGo(func() {
		defer close(ch)
		wg := &sync.WaitGroup{}

		for _, id := range opts.IDs {
			log.Infof(ctx, "[Send] Send files to %s", id)
			wg.Add(1)
			utils.SentryGo(func(id string) func() {
				return func() {

					defer wg.Done()
					if err := c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
						for _, file := range opts.Files {
							err := c.doSendFileToWorkload(ctx, workload.Engine, workload.ID, file)
							ch <- &types.SendMessage{ID: id, Path: file.Filename, Error: logger.ErrWithTracing(ctx, err)}
						}
						return nil
					}); err != nil {
						ch <- &types.SendMessage{ID: id, Error: logger.ErrWithTracing(ctx, err)}
					}
				}
			}(id))

		}
		wg.Wait()
	})
	return ch, nil
}

func (c *Calcium) doSendFileToWorkload(ctx context.Context, engine engine.API, ID string, file types.LinuxFile) error {
	log.Infof(ctx, "[doSendFileToWorkload] Send file to %s:%s", ID, file.Filename)
	return errors.WithStack(engine.VirtualizationCopyTo(ctx, ID, file.Filename, file.Clone().Content, file.UID, file.GID, file.Mode))
}
