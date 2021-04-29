package calcium

import (
	"bytes"
	"context"
	"io"
	"sync"

	"github.com/pkg/errors"
	"github.com/projecteru2/core/engine"
	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Send send files to workload
func (c *Calcium) Send(ctx context.Context, opts *types.SendOptions) (chan *types.SendMessage, error) {
	logger := log.WithField("Calcium", "Send").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(errors.WithStack(err))
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
						for dst, content := range opts.Data {
							err := c.doSendFileToWorkload(ctx, workload.Engine, workload.ID, dst, bytes.NewBuffer(content), true, true)
							ch <- &types.SendMessage{ID: id, Path: dst, Error: logger.Err(err)}
						}
						return nil
					}); err != nil {
						ch <- &types.SendMessage{ID: id, Error: logger.Err(err)}
					}
				}
			}(id))

		}
		wg.Wait()
	})
	return ch, nil
}

func (c *Calcium) doSendFileToWorkload(ctx context.Context, engine engine.API, ID, dst string, content io.Reader, AllowOverwriteDirWithFile bool, CopyUIDGID bool) error {
	log.Infof(ctx, "[doSendFileToWorkload] Send file to %s:%s", ID, dst)
	log.Debugf(ctx, "[doSendFileToWorkload] remote path %s", dst)
	return errors.WithStack(engine.VirtualizationCopyTo(ctx, ID, dst, content, AllowOverwriteDirWithFile, CopyUIDGID))
}
