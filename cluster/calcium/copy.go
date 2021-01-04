package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/log"
	"github.com/projecteru2/core/types"
)

// Copy uses VirtualizationCopyFrom cp to copy specified things and send to remote
func (c *Calcium) Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error) {
	if err := opts.Validate(); err != nil {
		return nil, err
	}
	ch := make(chan *types.CopyMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		log.Infof("[Copy] Copy %d workloads files", len(opts.Targets))
		// workload one by one
		for id, paths := range opts.Targets {
			wg.Add(1)
			go func(id string, paths []string) {
				defer wg.Done()
				if err := c.withWorkloadLocked(ctx, id, func(ctx context.Context, workload *types.Workload) error {
					for _, path := range paths {
						resp, name, err := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, path)
						ch <- makeCopyMessage(id, name, path, err, resp)
					}
					return nil
				}); err != nil {
					ch <- makeCopyMessage(id, "", "", err, nil)
				}
			}(id, paths)
		}
		wg.Wait()
	}()
	return ch, nil
}
