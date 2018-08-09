package calcium

import (
	"context"
	"sync"

	"github.com/projecteru2/core/cluster"
	"github.com/projecteru2/core/types"
	log "github.com/sirupsen/logrus"
)

//Copy uses docker cp to copy specified things and send to remote
func (c *Calcium) Copy(ctx context.Context, opts *types.CopyOptions) (chan *types.CopyMessage, error) {
	ch := make(chan *types.CopyMessage)
	go func() {
		defer close(ch)
		wg := sync.WaitGroup{}
		log.Debugf("[Copy] Copy %d containers files", len(opts.Targets))
		// container one by one
		for cid, paths := range opts.Targets {
			wg.Add(1)
			go func(cid string, paths []string) {
				defer wg.Done()
				container, err := c.GetContainer(ctx, cid)
				if err != nil {
					log.Errorf("[Copy] Error when get container %s", cid)
					ch <- makeCopyMessage(cid, cluster.CopyFailed, "", "", err, nil)
					return
				}
				for _, path := range paths {
					wg.Add(1)
					go func(path string) {
						defer wg.Done()
						resp, stat, err := container.Engine.CopyFromContainer(ctx, container.ID, path)
						log.Debugf("[Copy] Docker cp stat: %v", stat)
						if err != nil {
							log.Errorf("[Copy] Error during CopyFromContainer: %v", err)
							ch <- makeCopyMessage(cid, cluster.CopyFailed, "", path, err, nil)
							return
						}
						ch <- makeCopyMessage(cid, cluster.CopyOK, stat.Name, path, nil, resp)
					}(path)
				}
			}(cid, paths)
		}
		wg.Wait()
	}()
	return ch, nil
}
