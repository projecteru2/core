package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Copy uses VirtualizationCopyFrom cp to copy specified things and send to remote
func (c *Calcium) Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error) {
	logger := log.WithFunc("calcium.Copy").WithField("opts", opts)
	if err := opts.Validate(); err != nil {
		logger.Error(ctx, err)
		return nil, err
	}

	ch := make(chan *types.CopyMessage)
	_ = c.pool.Invoke(func() {
		defer close(ch)

		wg := sync.WaitGroup{}
		wg.Add(len(opts.Targets))
		defer wg.Wait()
		logger.Infof(ctx, "Copy %d workloads files", len(opts.Targets))

		// workload one by one
		for ID, paths := range opts.Targets {
			_ = c.pool.Invoke(func(ID string, paths []string) func() {
				return func() {
					defer wg.Done()

					workload, err := c.GetWorkload(ctx, ID)
					if err != nil {
						for _, path := range paths {
							logger.Error(ctx, err)
							ch <- &types.CopyMessage{
								ID:    ID,
								Path:  path,
								Error: err,
							}
						}
						return
					}

					for _, path := range paths {
						content, uid, gid, mode, err := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, path)
						ch <- &types.CopyMessage{
							ID:    ID,
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
			}(ID, paths))
		}
	})
	return ch, nil
}
