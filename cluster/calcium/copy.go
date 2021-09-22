package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
	"github.com/projecteru2/core/utils"
)

// Copy uses VirtualizationCopyFrom cp to copy specified things and send to remote
func (c *Calcium) Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error) {
	logger := log.WithField("Calcium", "Copy").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		return nil, logger.Err(ctx, err)
	}

	ch := make(chan *types.CopyMessage)
	utils.SentryGo(func() {
		defer close(ch)

		wg := sync.WaitGroup{}
		log.Infof(ctx, "[Copy] Copy %d workloads files", len(opts.Targets))

		// workload one by one
		for id, paths := range opts.Targets {
			wg.Add(1)

			utils.SentryGo(func(id string, paths []string) func() {
				return func() {
					defer wg.Done()

					workload, err := c.GetWorkload(ctx, id)
					if err != nil {
						for _, path := range paths {
							ch <- &types.CopyMessage{
								ID:    id,
								Path:  path,
								Error: logger.Err(ctx, err),
							}
						}
						return
					}

					for _, path := range paths {
						content, uid, gid, mode, err := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, path)
						ch <- &types.CopyMessage{
							ID:    id,
							Path:  path,
							Error: err,
							LinuxFile: types.LinuxFile{
								Filename: path,
								Content:  content,
								UID:      uid,
								GID:      gid,
								Mode:     mode,
							},
						}
					}
				}
			}(id, paths))
		}
		wg.Wait()
	})
	return ch, nil
}
