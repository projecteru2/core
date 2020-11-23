package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

// Copy uses VirtualizationCopyFrom cp to copy specified things and send to remote
func (c *Calcium) Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error) {
	ch := make(chan *types.CopyMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		log.Infof("[Copy] Copy %d workloads files", len(opts.Targets))
		// workload one by one
		for cid, paths := range opts.Targets {
			wg.Add(1)
			go func(cid string, paths []string) {
				defer wg.Done()
				workload, err := c.GetWorkload(ctx, cid)
				if err != nil {
					log.Errorf("[Copy] Error when get workload %s, err %v", cid, err)
					ch <- makeCopyMessage(cid, cluster.CopyFailed, "", "", err, nil)
					return
				}
				for _, path := range paths {
					wg.Add(1)
					go func(path string) {
						defer wg.Done()
						resp, name, err := workload.Engine.VirtualizationCopyFrom(ctx, workload.ID, path)
						if err != nil {
							log.Errorf("[Copy] Error during CopyFromWorkload: %v", err)
							ch <- makeCopyMessage(cid, cluster.CopyFailed, "", path, err, nil)
							return
						}
						ch <- makeCopyMessage(cid, cluster.CopyOK, name, path, nil, resp)
					}(path)
				}
			}(cid, paths)
		}
		wg.Wait()
	}()
	return ch, nil
}
